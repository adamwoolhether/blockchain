package worker

import (
	"errors"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
)

// peerOperations handles finding new peers.
func (w *Worker) peerOperations() {
	w.evHandler("Worker: peerOperations: G started")
	defer w.evHandler("Worker: peerOperations: G completed")

	w.runPeersOperation()

	for {
		select {
		case <-w.ticker.C:
			if !w.isShutdown() {
				w.runPeersOperation()
			}
		case <-w.shut:
			w.evHandler("Worker: peerOperations: received shut signal")
			return
		}
	}
}

// runPeersOperation updates the peer list.
func (w *Worker) runPeersOperation() {
	w.evHandler("Worker: runPeersOperation: started")
	defer w.evHandler("Worker: runPeersOperation: completed")

	for _, pr := range w.state.RetrieveKnownPeers() {

		// Retrieve the status of this peer.
		peerStatus, err := w.state.NetRequestPeerStatus(pr)
		if err != nil {
			w.evHandler("worker: runPeersOperation: requestPeerStatus: %s: ERROR: %s", pr.Host, err)

			// Since this known peer is unavailable, remove them from the list.
			w.state.RemoveKnownPeer(pr)
		}

		// Add new peers to this nodes list.
		w.addNewPeers(peerStatus.KnownPeers)
	}

	// Share with peers that this node is available to participate in the network.
	w.state.NetSendNodeAvailableToPeers()
}

// addNewPeers takes the list of known peers and makes sure
// they are included in the node's list of known peers.
func (w *Worker) addNewPeers(knownPeers []peer.Peer) error {
	w.evHandler("Worker: runPeerUpdatesOperation: addNewPeers: started")
	defer w.evHandler("Worker: runPeerUpdatesOperation: addNewPeers: completed")

	for _, pr := range knownPeers {
		// Don't add this running node to the known peer list.
		if pr.Match(w.state.RetrieveHost()) {
			return errors.New("already exists")
		}

		if w.state.AddKnownPeer(pr) {
			w.evHandler("Worker: runPeerUpdatesOperation: addNewPeers: add peer nodes: adding peer-node %s", pr)
		}
	}

	return nil
}
