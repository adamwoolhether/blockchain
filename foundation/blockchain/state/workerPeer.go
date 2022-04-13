package state

import (
	"errors"
	"fmt"
	"net/http"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
)

// peerOperations handles finding new peers.
func (w *worker) peerOperations() {
	w.evHandler("worker: peerOperations: G started")
	defer w.evHandler("worker: peerOperations: G completed")
	
	for {
		select {
		case <-w.ticker.C:
			if !w.isShutdown() {
				w.runPeersOperation()
			}
		case <-w.shut:
			w.evHandler("worker: peerOperations: received shut signal")
			return
		}
	}
}

// runPeersOperation updates the peer list.
func (w *worker) runPeersOperation() {
	w.evHandler("worker: runPeersOperation: started")
	defer w.evHandler("worker: runPeersOperation: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		
		// Retrieve the status of this peer.
		peerStatus, err := w.queryPeerStatus(pr)
		if err != nil {
			w.evHandler("worker: runPeersOperation: queryPeerStatus: %s: ERROR: %s", pr.Host, err)
		}
		
		// Add new peers to this nodes list.
		if err := w.addNewPeers(peerStatus.KnownPeers); err != nil {
			w.evHandler("worker: runPeersOperation: addNewPeers: %s: ERROR: %s", pr.Host, err)
		}
	}
}

// queryPeerStatus looks for new nodes on the blockchain by asking
// known nodes for their peer list. New nodes are added to the list.
func (w *worker) queryPeerStatus(pr peer.Peer) (peer.Status, error) {
	w.evHandler("worker: runPeerUpdatesOperations: queryPeerStatus: started: %s", pr)
	defer w.evHandler("worker: runPeerUpdatesOperations: queryPeerStatus: completed: %s", pr)
	
	url := fmt.Sprintf("%s/status", fmt.Sprintf(w.baseURL, pr.Host))
	
	var ps peer.Status
	if err := send(http.MethodGet, url, nil, &ps); err != nil {
		return peer.Status{}, err
	}
	
	w.evHandler("worker: runPeerUpdatesOperations: queryPeerStatus: peer-node[%s]: latest-blknum[%d]: peer-list[%s]", pr, ps.LatestBlockNumber, ps.KnownPeers)
	
	return ps, nil
}

// addNewPeers takes the list of known peers and makes sure
// they are included in the node's list of known peers.
func (w *worker) addNewPeers(knownPeers []peer.Peer) error {
	w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: started")
	defer w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: completed")
	
	for _, pr := range knownPeers {
		// Don't add this running node to the known peer list.
		if pr.Match(w.state.host) {
			return errors.New("already exists")
		}
		
		w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: add peer nodes: adding peer-node %s", pr)
		w.state.knownPeers.Add(pr)
	}
	
	return nil
}
