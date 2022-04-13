package state

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

// signalShareTransactions queues up a share transaction operation. If
// maxTxShareRequests signals exist in the channel, we won't send these.
func (w *worker) signalShareTransactions(blockTx storage.BlockTx) {
	select {
	case w.txSharing <- blockTx:
		w.evHandler("worker: signalShareTransactions: share Tx signaled")
	default:
		w.evHandler("worker: signalShareTransactions: queue full, transactions won't be shared.")
	}
}

// shareTxOperations handles sharing new user transactions.
func (w *worker) shareTxOperations() {
	w.evHandler("worker: shareTxOperations: G started")
	defer w.evHandler("worker: shareTxOperations: G completed")
	
	for {
		select {
		case tx := <-w.txSharing:
			if !w.isShutdown() {
				w.runShareTxOperation(tx)
			}
		case <-w.shut:
			w.evHandler("worker: shareTxOperations: received shut signal")
			return
		}
	}
}

// runShareTxOperation updates the peer list and sync's up the database.
func (w *worker) runShareTxOperation(tx storage.BlockTx) {
	w.evHandler("worker: runShareTxOperation: started")
	defer w.evHandler("worker: runShareTxOperation: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		url := fmt.Sprintf("%s/tx/submit", fmt.Sprintf(w.baseURL, pr.Host))
		if err := send(http.MethodPost, url, tx, nil); err != nil {
			w.evHandler("worker: runShareTxOperation: WARNING: %s", err)
		}
	}
}
