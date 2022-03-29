package v1

import (
	"net/http"
	
	"go.uber.org/zap"
	
	"github.com/adamwoolhether/blockchain/app/services/node/handlers/v1/public"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

const version = "v1"

// Config contains all mandatory systems required by handlers
type Config struct {
	Log *zap.SugaredLogger
}

// PublicRoutes binds all version 1 public routes.
func PublicRoutes(app *web.App, cfg Config) {
	pbl := public.Handlers{
		Log: cfg.Log,
		// State: cfg.State,
	}
	
	app.Handle(http.MethodGet, version, "/genesis", pbl.Genesis)
}