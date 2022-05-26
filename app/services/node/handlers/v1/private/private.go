// Package private maintains the group of handlers for node to node access.
package private

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	v1 "github.com/adamwoolhether/blockchain/business/web/v1"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/nameservice"
	"github.com/adamwoolhether/blockchain/foundation/web"
)

// Handlers manages the set of bar ledger endpoints.
type Handlers struct {
	Log   *zap.SugaredLogger
	State *state.State
	NS    *nameservice.NameService
}

// SubmitNodeTransaction adds new node transactions to the mempool.
func (h Handlers) SubmitNodeTransaction(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v, err := web.GetValues(ctx)
	if err != nil {
		return web.NewShutdownError("web value missing from context")
	}

	var tx database.BlockTx
	if err := web.Decode(r, &tx); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}

	h.Log.Infow("add tran", "traceid", v.TraceID, "from:nonce", tx, "to", tx.ToID, "value", tx.Value, "tip", tx.Tip)
	if err := h.State.UpsertNodeTransaction(tx); err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	resp := struct {
		Status string `json:"status"`
	}{
		Status: "transactions added to mempool",
	}

	return web.Respond(ctx, w, resp, http.StatusOK)
}

// ProposeBlock takes a block from a peer, validates it,
// and adds it to the blockchain.
func (h Handlers) ProposeBlock(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	// Decode the JSON in the post call into a file system block.
	var blockData database.BlockData
	if err := web.Decode(r, &blockData); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}

	// Convert the block data into a block. This creates a merkle tree
	// for the set of transactions required for blockchain operations.
	block, err := database.ToBlock(blockData)
	if err != nil {
		return fmt.Errorf("unable to decodde block: %w")
	}

	if err := h.State.ProcessProposedBlock(block); err != nil {
		if errors.Is(err, database.ErrChainForked) {
			h.State.Resync()
		}

		return v1.NewRequestError(errors.New("block not accepted"), http.StatusNotAcceptable)
	}

	resp := struct {
		Status string `json:"status"`
	}{
		Status: "accepted",
	}

	return web.Respond(ctx, w, resp, http.StatusOK)
}

// Status returns the current status of the node.
func (h Handlers) Status(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	latestBlock := h.State.RetrieveLatestBlock()

	status := peer.Status{
		LatestBlockHash:   latestBlock.Hash(),
		LatestBlockNumber: latestBlock.Header.Number,
		KnownPeers:        h.State.RetrieveKnownPeers(),
	}

	return web.Respond(ctx, w, status, http.StatusOK)
}

// BlocksByNumber returns all the blocks based on the specified to/from values.
func (h Handlers) BlocksByNumber(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	fromStr := web.Param(r, "from")
	if fromStr == "latest" || fromStr == "" {
		fromStr = fmt.Sprintf("%d", state.QueryLatest)
	}

	toStr := web.Param(r, "to")
	if toStr == "latest" || toStr == "" {
		toStr = fmt.Sprintf("%d", state.QueryLatest)
	}

	from, err := strconv.ParseUint(fromStr, 10, 64)
	if err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}
	to, err := strconv.ParseUint(toStr, 10, 64)
	if err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	if from > to {
		return v1.NewRequestError(errors.New("from greater than to"), http.StatusBadRequest)
	}

	blocks := h.State.QueryBlocksByNumber(from, to)
	if len(blocks) == 0 {
		return web.Respond(ctx, w, nil, http.StatusNoContent)
	}

	blockData := make([]database.BlockData, len(blocks))
	for i, block := range blocks {
		blockData[i] = database.NewBlockData(block)
	}

	return web.Respond(ctx, w, blockData, http.StatusOK)
}

// Mempool returns the set of uncommitted transactions.
func (h Handlers) Mempool(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	txs := h.State.RetrieveMempool()

	return web.Respond(ctx, w, txs, http.StatusOK)
}
