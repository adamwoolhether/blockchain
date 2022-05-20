package worker

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
)

// peerOperations handles finding new peers.
func (w *Worker) peerOperations() {
	w.evHandler("Worker: peerOperations: G started")
	defer w.evHandler("Worker: peerOperations: G completed")

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
		peerStatus, err := w.queryPeerStatus(pr)
		if err != nil {
			w.evHandler("Worker: runPeersOperation: queryPeerStatus: %s: ERROR: %s", pr.Host, err)
		}

		// Add new peers to this nodes list.
		w.addNewPeers(peerStatus.KnownPeers)
	}
}

// queryPeerStatus looks for new nodes on the blockchain by asking
// known nodes for their peer list. New nodes are added to the list.
func (w *Worker) queryPeerStatus(pr peer.Peer) (peer.Status, error) {
	w.evHandler("Worker: runPeerUpdatesOperations: queryPeerStatus: started: %s", pr)
	defer w.evHandler("Worker: runPeerUpdatesOperations: queryPeerStatus: completed: %s", pr)

	url := fmt.Sprintf("%s/status", fmt.Sprintf(w.baseURL, pr.Host))

	var ps peer.Status
	if err := send(http.MethodGet, url, nil, &ps); err != nil {
		return peer.Status{}, err
	}

	w.evHandler("Worker: runPeerUpdatesOperations: queryPeerStatus: peer-node[%s]: latest-blknum[%d]: peer-list[%s]", pr, ps.LatestBlockNumber, ps.KnownPeers)

	return ps, nil
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
