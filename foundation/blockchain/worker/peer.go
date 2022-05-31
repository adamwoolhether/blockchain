package worker

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
)

// CORE NOTE: The p2p network is managed by this goroutine. There is
// a single node that is considered the origin node. The defaults in
// main.go represent the origin node. That node must be running first.
// All new peer nodes connect to the origin node to identify all other
// peers on the network. The topology is all nodes having a connection
// to all other nodes. If a node does not respond to a network call,
// they are removed from the peer list until the next peer operation.

// peerOperations handles finding new peers.
func (w *Worker) peerOperations() {
	w.evHandler("Worker: peerOperations: G started")
	defer w.evHandler("Worker: peerOperations: G completed")

	// On startup talk to the origin node and get an updated
	// peers list. Then share with the network that this node
	// is available for transaction and block submissions.
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

			// Since this peer is unavailable, remove them from the list.
			w.state.RemoveKnownPeer(pr)
		}

		// Add peers from this node's peer list that are currently missing.
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
			continue
		}

		// Log if the peer is new.
		if w.state.AddKnownPeer(pr) {
			w.evHandler("Worker: runPeerUpdatesOperation: addNewPeers: add peer nodes: adding peer-node %s", pr.Host)
		}
	}

	return nil
}
