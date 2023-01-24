package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"go.uber.org/zap"

	"github.com/adamwoolhether/blockchain/app/services/viewer/handlers"
	"github.com/adamwoolhether/blockchain/foundation/logger"
)

var build = "develop"

func main() {
	log, err := logger.New("VIEWER")
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer log.Sync()

	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		log.Sync()
		os.Exit(1)
	}
}

func run(log *zap.SugaredLogger) error {
	// /////////////////////////////////////////////////////////////
	// Configuration

	cfg := struct {
		conf.Version
		Web struct {
			UIHost          string        `conf:"default:0.0.0.0:80"`
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:10s"`
			IdleTimeout     time.Duration `conf:"default:120s"`
			ShutdownTimeout time.Duration `conf:"default:20s"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "copyright info",
		},
	}

	const prefix = "VIEWER"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// /////////////////////////////////////////////////////////////
	// App Start
	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown completed")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	// /////////////////////////////////////////////////////////////
	// Service Start/Stop Support
	log.Infow("startup", "status", "initializing viewer")

	// Make a channel to listen for an interrupt or terminate signal for OS.
	// Use buffered chanel, as signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Load template and bind handlers.
	uiMux, err := handlers.UIMux(build, shutdown, log)
	if err != nil {
		return fmt.Errorf("unable to bind handlers: %w", err)
	}

	// Create server to handle and route traffic.
	viewer := http.Server{
		Addr:         cfg.Web.UIHost,
		Handler:      uiMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
	}

	// Make channel to listen for errors coming from listener. User buffered channel
	// so the goroutine can exit if we don't collect the error.
	serverErrors := make(chan error, 1)

	// Start the service listening for requests.
	go func() {
		log.Infow("startup", "status", "viewer service started", "host", viewer.Addr)
		serverErrors <- viewer.ListenAndServe()
	}()

	// /////////////////////////////////////////////////////////////
	// Shutdown

	// Block main and wait for shutdown.
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		// Give outstanding requests deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Ask listener to shut down and shed load.
		log.Infow("shutdown", "status", "shutdown viewer service started")
		if err := viewer.Shutdown(ctx); err != nil {
			viewer.Close()
			return fmt.Errorf("could not stop viewer service gracefully: %w", err)
		}
	}

	return nil
}