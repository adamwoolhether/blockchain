// Package handlers contains the full set of handler functions and routes
// supported by the web api.
package handlers

import (
	"context"
	"net/http"
	"os"

	"go.uber.org/zap"

	"github.com/adamwoolhether/blockchain/business/web/v1/mid"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

func UIMux(build string, shutdown chan os.Signal, log *zap.SugaredLogger) (*web.App, error) {
	app := web.NewApp(shutdown, mid.Logger(log), mid.Errors(log), mid.Panics(), mid.Cors("*"))

	// Register the index page for the website.
	app.Handle(http.MethodGet, "", "/", handler)

	// Register the assets.
	fs := http.FileServer(http.Dir("app/services/viewer/assets"))
	fs = http.StripPrefix("/assets/", fs)
	f := func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		fs.ServeHTTP(w, r)
		return nil
	}
	app.Handle(http.MethodGet, "", "/assets/*", f)

	return app, nil
}
