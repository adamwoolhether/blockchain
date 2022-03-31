// Package public maintains the group of handlers for public access.
package public

import (
	"context"
	"fmt"
	"net/http"
	
	"go.uber.org/zap"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
	"github.com/adamwoolhether/blockchain/foundation/nameservice"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

// Handlers manages the set of bar ledger endpoints.
type Handlers struct {
	Log   *zap.SugaredLogger
	State *state.State
	NS    *nameservice.NameService
}

// SubmitWalletTransaction adds a new user transaction to the mempool.
func (h Handlers) SubmitWalletTransaction(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v, err := web.GetValues(ctx)
	if err != nil {
		return err
	}
	
	var signedTx storage.SignedTx
	if err := web.Decode(r, &signedTx); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}
	
	from, err := signedTx.FromAccount()
	if err != nil {
		return fmt.Errorf("unable to get from account address: %w", err)
	}
	
	h.Log.Infow("add user tran", "traceid", v.TraceID, "nonce", signedTx.Nonce, "from", from, "to", signedTx.To, "value", signedTx.Value, "tip", signedTx.Tip)
	if err := h.State.SubmitWalletTransaction(signedTx); err != nil {
		return err
	}
	
	resp := struct {
		Status string `json:"status"`
	}{
		Status: "TX SUCCESSFUL",
	}
	
	return web.Respond(ctx, w, resp, http.StatusOK)
}

// Mempool returns the set of uncommited transactions.
func (h Handlers) Mempool(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	mpool := h.State.RetrieveMempool()
	
	return web.Respond(ctx, w, mpool, http.StatusOK)
}

// Genesis returns the genesis block information.
func (h Handlers) Genesis(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	gen := h.State.RetrieveGenesis()
	
	return web.Respond(ctx, w, gen, http.StatusOK)
}

// Accounts returns the current balances for all users.
func (h Handlers) Accounts(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	blkAccounts := h.State.RetrieveAccounts()
	
	accts := make([]info, 0, len(blkAccounts))
	for account, blkInfo := range blkAccounts {
		acct := info{
			Account: account,
			Name:    h.NS.Lookup(account),
			Balance: blkInfo.Balance,
		}
		accts = append(accts, acct)
	}
	
	ai := acctInfo{
		Uncommitted: len(h.State.RetrieveMempool()),
		Accounts:    accts,
	}
	
	return web.Respond(ctx, w, ai, http.StatusOK)
}
