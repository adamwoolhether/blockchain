// Package v1 contains the full set of handler functions and
// routes supported by the v1 web api.
package v1

import (
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/adamwoolhether/blockchain/app/services/node/handlers/v1/private"
	"github.com/adamwoolhether/blockchain/app/services/node/handlers/v1/public"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/events"
	"github.com/adamwoolhether/blockchain/foundation/nameservice"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

const version = "v1"

// Config contains all mandatory systems required by handlers
type Config struct {
	Log   *zap.SugaredLogger
	State *state.State
	WS    websocket.Upgrader
	NS    *nameservice.NameService
	Evts  *events.Events
}

// PublicRoutes binds all the version 1 public routes.
func PublicRoutes(app *web.App, cfg Config) {
	pbl := public.Handlers{
		Log:   cfg.Log,
		State: cfg.State,
		WS:    websocket.Upgrader{},
		NS:    cfg.NS,
		Evts:  cfg.Evts,
	}

	app.Handle(http.MethodGet, version, "/events", pbl.Events)
	app.Handle(http.MethodGet, version, "/genesis/list", pbl.Genesis)
	app.Handle(http.MethodGet, version, "/accounts/list", pbl.Accounts)
	app.Handle(http.MethodGet, version, "/accounts/list/:account", pbl.Accounts)
	app.Handle(http.MethodGet, version, "/blocks/list", pbl.BlocksByAccount)
	app.Handle(http.MethodGet, version, "/blocks/list/:account", pbl.BlocksByAccount)
	app.Handle(http.MethodGet, version, "/tx/uncommitted/list", pbl.Mempool)
	app.Handle(http.MethodGet, version, "/tx/uncommitted/list/:account", pbl.Mempool)
	app.Handle(http.MethodPost, version, "/tx/submit", pbl.SubmitWalletTransaction)
	app.Handle(http.MethodPost, version, "/tx/proof/:block/", pbl.SubmitWalletTransaction)
}

// PrivateRoutes binds all the version 1 private routes.
func PrivateRoutes(app *web.App, cfg Config) {
	prv := private.Handlers{
		Log:   cfg.Log,
		State: cfg.State,
		NS:    cfg.NS,
	}

	app.Handle(http.MethodPost, version, "/node/peers", prv.SubmitPeer)
	app.Handle(http.MethodGet, version, "/node/status", prv.Status)
	app.Handle(http.MethodGet, version, "/node/block/list/:from/:to", prv.BlocksByNumber)
	app.Handle(http.MethodPost, version, "/node/block/next", prv.ProposeBlock)
	app.Handle(http.MethodPost, version, "/node/tx/submit", prv.SubmitNodeTransaction)
	app.Handle(http.MethodGet, version, "/node/tx/list", prv.Mempool)
}
