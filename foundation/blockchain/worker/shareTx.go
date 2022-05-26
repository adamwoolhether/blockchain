package worker

import (
	"fmt"
	"net/http"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// maxTxShareRequests represents the max number of pending-tx network
// share requests that can be outstanding before share requests are dropped.
// ToID keep this simple, a buffered channel of this arbitrary number is being
// used. If the channel becomes full, requests for new transactions to be
// shared will not be accepted. This isn't production friendly.
const maxTxShareRequests = 100

// shareTxOperations handles sharing new user transactions.
func (w *Worker) shareTxOperations() {
	w.evHandler("Worker: shareTxOperations: G started")
	defer w.evHandler("Worker: shareTxOperations: G completed")

	for {
		select {
		case tx := <-w.txSharing:
			if !w.isShutdown() {
				w.runShareTxOperation(tx)
			}
		case <-w.shut:
			w.evHandler("Worker: shareTxOperations: received shut signal")
			return
		}
	}
}

// runShareTxOperation updates the peer list and sync's up the database.
func (w *Worker) runShareTxOperation(tx database.BlockTx) {
	w.evHandler("Worker: runShareTxOperation: started")
	defer w.evHandler("Worker: runShareTxOperation: completed")

	// CORE NOTE: Bitcoin does not send the full transaction immediately to save on
	// bandwidth. A node will send the transaction's mempool key first so
	// the receiving node can check if they already have the transaction or
	// not. If the receiving node doesn't have it, then it will request the
	// transaction based on the mempool key it received.

	// For now, the Ardan blockchain just sends the full transaction.
	for _, pr := range w.state.RetrieveKnownPeers() {
		url := fmt.Sprintf("%s/tx/submit", fmt.Sprintf(w.baseURL, pr.Host))
		if err := send(http.MethodPost, url, tx, nil); err != nil {
			w.evHandler("Worker: runShareTxOperation: WARNING: %s", err)
		}
	}
}
