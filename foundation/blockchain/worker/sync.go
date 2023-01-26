package worker

// CORE NOTE: On startup or when reorganizing the chain, the node needs to be
// in sync with the rest of the network. This includes the mempool and
// blockchain database. This operation needs to finish before the node can
// participate in the network.

// Sync updates the peer list, mempool, and blocks.
func (w *Worker) Sync() {
	w.evHandler("Worker: sync: started")
	defer w.evHandler("Worker: sync: completed")

	for _, pr := range w.state.KnownExternalPeers() {
		// Retrieve the status of this peer.
		peerStatus, err := w.state.NetRequestPeerStatus(pr)
		if err != nil {
			w.evHandler("Worker: sync: queryPeerStatus: %s: ERROR: %s", pr.Host, err)
		}

		// Add new peers to this nodes list.
		w.addNewPeers(peerStatus.KnownPeers)

		// Update the mempool.
		pool, err := w.state.NetRequestPeerMempool(pr)
		if err != nil {
			w.evHandler("Worker: sync: retrievePeerMempool: %s: ERROR: %s", pr.Host, err)
		}
		for _, tx := range pool {
			w.evHandler("Worker: sync: retrievePeerMempool: %s: Add Tx: %s", pr.Host, tx.SignatureString()[:16])
			w.state.UpsertMempool(tx)
		}

		// If this peer has blocks we don't have, we need to add them.
		if peerStatus.LatestBlockNumber > w.state.LatestBlock().Header.Number {
			w.evHandler("Worker: sync: writePeerBlocks: %s: latestBlockNumber[%d]", pr.Host, peerStatus.LatestBlockNumber)

			if err := w.state.NetRequestPeerBlocks(pr); err != nil {
				w.evHandler("Worker: sync: writePeerBlocks: %s: ERROR %s", pr.Host, err)
			}
		}
	}

	// Share with peers that this node is available to participate in the network.
	w.state.NetSendNodeAvailableToPeers()
}
