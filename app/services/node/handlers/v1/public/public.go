// Package public maintains the group of handlers for public access.
package public

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"go.uber.org/zap"
	
	"github.com/gorilla/websocket"
	
	v1 "github.com/adamwoolhether/blockchain/business/web/v1"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/events"
	"github.com/adamwoolhether/blockchain/foundation/nameservice"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

// Handlers manages the set of bar ledger endpoints.
type Handlers struct {
	Log   *zap.SugaredLogger
	State *state.State
	WS    websocket.Upgrader
	NS    *nameservice.NameService
	Evts  *events.Events
}

// Events handles a web socket to provide events to a client.
func (h Handlers) Events(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v, err := web.GetValues(ctx)
	if err != nil {
		return err
	}
	
	h.WS.CheckOrigin = func(r *http.Request) bool { return true } // required to bypass CORS issues, this is a security issue!.
	
	// "hijack"" the http connection to a websocket connection
	c, err := h.WS.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	
	ch := h.Evts.Acquire(v.TraceID)
	defer h.Evts.Release(v.TraceID)
	
	ticker := time.NewTicker(time.Second)
	
	for {
		select {
		case msg, wd := <-ch:
			if !wd {
				return nil
			}
			if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return err
			}
		case <-ticker.C:
			if err := c.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				return nil
			}
		}
	}
}

// SubmitWalletTransaction adds a new user transaction to the mempool.
func (h Handlers) SubmitWalletTransaction(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v, err := web.GetValues(ctx)
	if err != nil {
		return err
	}
	
	var signedTx database.SignedTx
	if err := web.Decode(r, &signedTx); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}
	
	h.Log.Infow("add user tran", "traceid", v.TraceID, "from:nonce", signedTx, "to", signedTx.ToID, "value", signedTx.Value, "tip", signedTx.Tip)
	if err := h.State.UpsertWalletTransaction(signedTx); err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}
	
	resp := struct {
		Status string `json:"status"`
	}{
		Status: "transactions added to mempool",
	}
	
	return web.Respond(ctx, w, resp, http.StatusOK)
}

// Genesis return the genesis block information.
func (h Handlers) Genesis(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	gen := h.State.RetrieveGenesis()
	
	return web.Respond(ctx, w, gen, http.StatusOK)
}

// Mempool returns the set of uncommited transactions.
func (h Handlers) Mempool(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	acct := web.Param(r, "account")
	
	mpool := h.State.RetrieveMempool()
	
	txs := []tx{}
	for _, t := range mpool {
		account, _ := t.FromAccount()
		if acct != "" && ((acct != string(account)) && (acct != string(t.ToID))) {
			continue
		}
		
		txs = append(txs, tx{
			FromAccount: account,
			FromName:    h.NS.Lookup(account),
			To:          t.ToID,
			ToName:      h.NS.Lookup(t.ToID),
			Nonce:       t.Nonce,
			Value:       t.Value,
			Tip:         t.Tip,
			Data:        t.Data,
			TimeStamp:   t.TimeStamp,
			Gas:         t.Gas,
			Sig:         t.SignatureString(),
		})
	}
	
	return web.Respond(ctx, w, mpool, http.StatusOK)
}

// Accounts returns the current balances for all users.
func (h Handlers) Accounts(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	accountStr := web.Param(r, "accountID")
	
	var accounts map[database.AccountID]database.Account
	switch accountStr {
	case "":
		accounts = h.State.RetrieveAccounts()
	default:
		accountID, err := database.ToAccountID(accountStr)
		if err != nil {
			return err
		}
		account, err := h.State.QueryAccounts(accountID)
		if err != nil {
			return err
		}
		
		accounts = map[database.AccountID]database.Account{accountID: account}
	}
	
	resp := make([]acct, 0, len(accounts))
	for account, info := range accounts {
		acct := acct{
			Account: account,
			Name:    h.NS.Lookup(account),
			Balance: info.Balance,
			Nonce:   info.Nonce,
		}
		resp = append(resp, acct)
	}
	
	ai := acctInfo{
		LatestBlock: h.State.RetrieveLatestBlock().Hash(),
		Uncommitted: len(h.State.RetrieveMempool()),
		Accounts:    resp,
	}
	
	return web.Respond(ctx, w, ai, http.StatusOK)
}

// BlocksByAccount returns all the blocks and their details.
func (h Handlers) BlocksByAccount(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	accountStr, err := database.ToAccountID(web.Param(r, "accountStr"))
	if err != nil {
		return err
	}
	
	dbBlocks := h.State.QueryBlocksByAccount(accountStr)
	if len(dbBlocks) == 0 {
		return web.Respond(ctx, w, nil, http.StatusNoContent)
	}
	
	blocks := make([]block, len(dbBlocks))
	for j, blk := range dbBlocks {
		values := blk.Transactions.Values()
		txs := make([]tx, len(blk.Transactions.Values()))
		
		for i, tran := range values {
			account, err := tran.FromAccount()
			if err != nil {
				return err
			}
			
			txs[i] = tx{
				FromAccount: account,
				FromName:    h.NS.Lookup(account),
				To:          tran.ToID,
				ToName:      h.NS.Lookup(tran.ToID),
				Nonce:       tran.Nonce,
				Value:       tran.Value,
				Tip:         tran.Tip,
				Data:        tran.Data,
				TimeStamp:   tran.TimeStamp,
				Gas:         tran.Gas,
				Sig:         tran.SignatureString(),
			}
		}
		
		b := block{
			ParentHash:   blk.Header.ParentHash,
			MinerAccount: blk.Header.MinerAccountID,
			Difficulty:   blk.Header.Difficulty,
			Number:       blk.Header.Number,
			TimeStamp:    blk.Header.TimeStamp,
			Nonce:        blk.Header.Nonce,
			Transactions: txs,
		}
		
		blocks[j] = b
	}
	
	return web.Respond(ctx, w, blocks, http.StatusOK)
}
