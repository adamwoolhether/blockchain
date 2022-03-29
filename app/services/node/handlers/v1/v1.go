// Package v1 contains the full set of handler functions and
// routes supported by the v1 web api.
package v1

import (
	"net/http"
	
	"go.uber.org/zap"
	
	"github.com/adamwoolhether/blockchain/app/services/node/handlers/v1/public"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

const version = "v1"

// Config contains all mandatory systems required by handlers
type Config struct {
	Log   *zap.SugaredLogger
	State *state.State
}

// PublicRoutes binds all version 1 public routes.
func PublicRoutes(app *web.App, cfg Config) {
	pbl := public.Handlers{
		Log:   cfg.Log,
		State: cfg.State,
	}
	
	app.Handle(http.MethodGet, version, "/tx/uncommitted/list", pbl.Mempool)
	app.Handle(http.MethodPost, version, "/tx/submit", pbl.SubmitWalletTransaction)
	app.Handle(http.MethodGet, version, "/genesis", pbl.Genesis)
	app.Handle(http.MethodGet, version, "/accounts/list", pbl.Accounts)
}
