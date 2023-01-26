// Package public maintains the group of handlers for public access.
package public

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"

	"github.com/ethereum/go-ethereum/common/hexutil"

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

// SubmitWalletTransaction adds a new transaction to the mempool.
func (h Handlers) SubmitWalletTransaction(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v, err := web.GetValues(ctx)
	if err != nil {
		return err
	}

	var signedTx database.SignedTx
	if err := web.Decode(r, &signedTx); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}

	h.Log.Infow("add tran", "traceid", v.TraceID, "sig:nonce", signedTx, "from", signedTx.FromID, "to", signedTx.ToID, "value", signedTx.Value, "tip", signedTx.Tip)

	// Ask the state package to add this transaction to the mempool. Only the
	// checks are the transaction signature and the recipient account format.
	// It's up to the wallet to make sure the account has a proper balance and
	// nonce. Fees will be taken if this transaction is mined into a block.
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

	txs := make([]tx, 0, len(mpool))
	for _, t := range mpool {
		if acct != "" && ((acct != string(t.FromID)) && (acct != string(t.ToID))) {
			continue
		}

		txs = append(txs, tx{
			FromAccount: t.FromID,
			FromName:    h.NS.Lookup(t.FromID),
			To:          t.ToID,
			ToName:      h.NS.Lookup(t.ToID),
			ChainID:     t.ChainID,
			Nonce:       t.Nonce,
			Value:       t.Value,
			Tip:         t.Tip,
			Data:        t.Data,
			TimeStamp:   t.TimeStamp,
			GasPrice:    t.GasPrice,
			GasUnits:    t.GasUnits,
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
		account, err := h.State.QueryAccount(accountID)
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
	var accountID database.AccountID
	accountStr := web.Param(r, "account")
	if accountStr != "" {
		var err error
		accountID, err = database.ToAccountID(web.Param(r, "account"))
		if err != nil {
			return err
		}
	}

	dbBlocks, err := h.State.QueryBlocksByAccount(accountID)
	if err != nil {
		return err
	}
	if len(dbBlocks) == 0 {
		return web.Respond(ctx, w, nil, http.StatusNoContent)
	}

	blocks := make([]block, len(dbBlocks))
	for j, blk := range dbBlocks {
		values := blk.MerkleTree.Values()

		txs := make([]tx, len(values))
		for i, tran := range values {
			rawProof, order, err := blk.MerkleTree.Proof(tran)
			if err != nil {
				return err
			}
			proof := make([]string, len(rawProof))
			for i, rp := range rawProof {
				proof[i] = hexutil.Encode(rp)
			}

			txs[i] = tx{
				FromAccount: tran.FromID,
				FromName:    h.NS.Lookup(tran.FromID),
				To:          tran.ToID,
				ToName:      h.NS.Lookup(tran.ToID),
				ChainID:     tran.ChainID,
				Nonce:       tran.Nonce,
				Value:       tran.Value,
				Tip:         tran.Tip,
				Data:        tran.Data,
				TimeStamp:   tran.TimeStamp,
				GasPrice:    tran.GasPrice,
				GasUnits:    tran.GasUnits,
				Sig:         tran.SignatureString(),
				Proof:       proof,
				ProofOrder:  order,
			}
		}

		b := block{
			Number:        blk.Header.Number,
			PrevBlockHash: blk.Header.PrevBlockHash,
			TimeStamp:     blk.Header.TimeStamp,
			BeneficiaryID: blk.Header.BeneficiaryID,
			Difficulty:    blk.Header.Difficulty,
			MiningReward:  blk.Header.MiningReward,
			Nonce:         blk.Header.Nonce,
			StateRoot:     blk.Header.StateRoot,
			TransRoot:     blk.Header.TransRoot,
			Transactions:  txs,
		}

		blocks[j] = b
	}

	return web.Respond(ctx, w, blocks, http.StatusOK)
}
