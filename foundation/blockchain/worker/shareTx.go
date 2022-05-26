package worker

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
				w.state.NetSendTxToPeers(tx)
			}
		case <-w.shut:
			w.evHandler("Worker: shareTxOperations: received shut signal")
			return
		}
	}
}
