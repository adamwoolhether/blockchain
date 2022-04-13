package worker

import (
	"fmt"
	"net/http"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// maxTxShareRequests represents the max number of pending-tx network
// share requests that can be outstanding before share requests are dropped.
// To keep this simple, a buffered channel of this arbitrary number is being
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
func (w *Worker) runShareTxOperation(tx storage.BlockTx) {
	w.evHandler("Worker: runShareTxOperation: started")
	defer w.evHandler("Worker: runShareTxOperation: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		url := fmt.Sprintf("%s/tx/submit", fmt.Sprintf(w.baseURL, pr.Host))
		if err := send(http.MethodPost, url, tx, nil); err != nil {
			w.evHandler("Worker: runShareTxOperation: WARNING: %s", err)
		}
	}
}
