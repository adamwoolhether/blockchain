package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/adamwoolhether/blockchain/app/services/node/handlers"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage/disk"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/worker"
	"github.com/adamwoolhether/blockchain/foundation/events"
	"github.com/adamwoolhether/blockchain/foundation/logger"
	"github.com/adamwoolhether/blockchain/foundation/nameservice"
)

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	// Construct app logger.
	log, err := logger.New("NODE")
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer log.Sync()

	// Perform the startup and shutdown sequence.
	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		log.Sync()
		os.Exit(1)
	}
}

func run(log *zap.SugaredLogger) error {
	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Configuration
	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:10s"`
			IdleTimeout     time.Duration `conf:"default:120s"`
			ShutdownTimeout time.Duration `conf:"default:20s"`
			PublicHost      string        `conf:"default:0.0.0.0:8080"`
			PrivateHost     string        `conf:"default:0.0.0.0:9080"`
		}
		State struct {
			Beneficiary    string   `conf:"default:miner1"`
			DBPath         string   `conf:"default:zblock/miner1/"`
			SelectStrategy string   `conf:"default:Tip"`
			OriginPeers    []string `conf:"default:0.0.0.0:9080"`
			Consensus      string   `conf:"default:POW"` // Change to POA to run Proof of Authority
		}
		NameService struct {
			Folder string `conf:"default:zblock/accounts/"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "copyright information here",
		},
	}

	const prefix = "NODE"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}

		return fmt.Errorf("parsing config: %w", err)
	}

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// App Starting
	var header = `
 █████╗ ██████╗ ██████╗  █████╗ ███╗   ██╗    ██████╗ ██╗      ██████╗  ██████╗██╗  ██╗ ██████╗██╗  ██╗ █████╗ ██╗███╗   ██╗
██╔══██╗██╔══██╗██╔══██╗██╔══██╗████╗  ██║    ██╔══██╗██║     ██╔═══██╗██╔════╝██║ ██╔╝██╔════╝██║  ██║██╔══██╗██║████╗  ██║
███████║██████╔╝██║  ██║███████║██╔██╗ ██║    ██████╔╝██║     ██║   ██║██║     █████╔╝ ██║     ███████║███████║██║██╔██╗ ██║
██╔══██║██╔══██╗██║  ██║██╔══██║██║╚██╗██║    ██╔══██╗██║     ██║   ██║██║     ██╔═██╗ ██║     ██╔══██║██╔══██║██║██║╚██╗██║
██║  ██║██║  ██║██████╔╝██║  ██║██║ ╚████║    ██████╔╝███████╗╚██████╔╝╚██████╗██║  ██╗╚██████╗██║  ██║██║  ██║██║██║ ╚████║
╚═╝  ╚═╝╚═╝  ╚═╝╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝    ╚═════╝ ╚══════╝ ╚═════╝  ╚═════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝`
	fmt.Println(header)

	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Name Service Support
	ns, err := nameservice.New(cfg.NameService.Folder)
	if err != nil {
		return fmt.Errorf("unable to load account name service: %w", err)
	}

	for account, name := range ns.Copy() {
		log.Infow("startup", "status", "nameservice", "name", name, "account", account)
	}

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Blockchain Support

	// Load the private key file for the configured beneficiary
	// so the account can get credited with fees and tips.
	path := fmt.Sprintf("%s%s.ecdsa", cfg.NameService.Folder, cfg.State.Beneficiary)
	privateKey, err := crypto.LoadECDSA(path)
	if err != nil {
		return fmt.Errorf("unable to load private key for node: %w", err)
	}

	peerSet := peer.NewSet()
	for _, host := range cfg.State.OriginPeers {
		peerSet.Add(peer.New(host))
	}
	peerSet.Add(peer.New(cfg.Web.PrivateHost))

	evts := events.New()
	ev := func(v string, args ...any) {
		const websocketPrefix = "viewer:"

		s := fmt.Sprintf(v, args...)
		log.Infow(s, "traceid", "00000000-0000-0000-0000-000000000000")
		if strings.HasPrefix(s, websocketPrefix) {
			evts.Send(s)
		}
	}

	// Construct disk storage.
	storage, err := disk.New(cfg.State.DBPath)
	if err != nil {
		return err
	}

	// Load genesis file for initial blockchain settings and origin balances.
	genesis, err := genesis.Load()
	if err != nil {
		return err
	}

	st, err := state.New(state.Config{
		BeneficiaryID:  database.PublicKeyToAccountID(privateKey.PublicKey),
		Host:           cfg.Web.PrivateHost,
		Storage:        storage,
		Genesis:        genesis,
		SelectStrategy: cfg.State.SelectStrategy,
		KnownPeers:     peerSet,
		Consensus:      cfg.State.Consensus,
		EvHandler:      ev,
	})
	if err != nil {
		return err
	}
	defer st.Shutdown()

	worker.Run(st, ev)

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Service Start/Stop Support

	// Make a channel to listen for an interrupt or terminal signal
	// from the OS. Signal package requires a buffered channel.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// User a buffered channel to listen for errors from listener. A buffered
	// channel is used so goroutine can exit if the error isn't collected.
	serverErrors := make(chan error, 1)

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Start Public Service
	log.Infow("startup", "status", "initializing V1 public API support")

	// Construct the mux for public API calls.
	publicMux := handlers.PublicMux(handlers.MuxConfig{
		Shutdown: shutdown,
		Log:      log,
		State:    st,
		NS:       ns,
		Evts:     evts,
	})

	// Construct a server to service the requests against the Mux.
	public := http.Server{
		Addr:         cfg.Web.PublicHost,
		Handler:      publicMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	// Start the service listening for api requests.
	go func() {
		log.Infow("startup", "status", "public api router started", "host", public.Addr)
		serverErrors <- public.ListenAndServe()
	}()

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Start Private Service
	log.Infow("startup", "status", "initializing V1 private API support")

	// Construct the mux for private API calls.
	privateMux := handlers.PrivateMux(handlers.MuxConfig{
		Shutdown: shutdown,
		Log:      log,
		State:    st,
	})

	// Construct a server to service the requests against the Mux.
	private := http.Server{
		Addr:         cfg.Web.PrivateHost,
		Handler:      privateMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	// Start the service listening for api requests.
	go func() {
		log.Infow("startup", "status", "private api router started", "host", private.Addr)
		serverErrors <- private.ListenAndServe()
	}()

	// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Shutdown

	// Block main waiting for shutdown.
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server errors: %w", err)
	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancelPub := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancelPub()

		// Asking listener to shut down and shed load.
		log.Infow("shutdown", "status", "shutdown private API started")
		if err := private.Shutdown(ctx); err != nil {
			private.Close()
			return fmt.Errorf("could not stop private service gracefully: %w", err)
		}

		// Give outstanding requests a deadline for completion.
		ctx, cancelPri := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancelPri()

		// Asking listener to shut down and shed load.
		log.Infow("shutdown", "status", "shutdown public API started")
		if err := public.Shutdown(ctx); err != nil {
			public.Close()
			return fmt.Errorf("could not stop public service gracefully: %w", err)
		}
	}

	return nil
}
