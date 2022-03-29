package public

import (
	"context"
	"net/http"
	
	"go.uber.org/zap"
	
	"github.com/adamwoolhether/blockchain/foundation/web"
)

// Handlers manages the set of bar ledger endpoints.
type Handlers struct {
	Log *zap.SugaredLogger
}

// Test adds new user transaction to the mempool.
func (h Handlers) Test(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	status := struct {
		Status string
	}{
		Status: "OK",
	}
	
	return web.Respond(ctx, w, status, http.StatusOK)
}
