package state

import (
	"fmt"
	"net/http"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// sync updates the peer list, mempool, and blocks.
func (w *worker) sync() {
	w.evHandler("worker: sync: started")
	defer w.evHandler("worker: sync: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		// Retrieve the status of this peer.
		peerStatus, err := w.queryPeerStatus(pr)
		if err != nil {
			w.evHandler("worker: sync: queryPeerStatus: %s: ERROR: %s", pr.Host, err)
		}
		
		// Add new peers to this nodes list.
		if err := w.addNewPeers(peerStatus.KnownPeers); err != nil {
			w.evHandler("worker: sync: addNewPeers: %s: ERROR: %s", pr.Host, err)
		}
		
		// Update the mempool.
		pool, err := w.retrievePeerMempool(pr)
		if err != nil {
			w.evHandler("worker: sync: retrievePeerMempool: %s: ERROR: %s", pr.Host, err)
		}
		for _, tx := range pool {
			w.evHandler("worker: sync: retrievePeerMempool: %s: Add Tx: %s", pr.Host, tx.SignatureString()[:16])
			w.state.mempool.Upsert(tx)
		}
		
		// If this peer has blocks we don't have, we need to add them.
		if peerStatus.LatestBlockNumber > w.state.RetrieveLatestBlock().Header.Number {
			w.evHandler("worker: sync: writePeerBlocks: %s: latestBlockNumber[%d]", pr.Host, peerStatus.LatestBlockNumber)
			
			if err := w.retrievePeerBlocks(pr); err != nil {
				w.evHandler("worker: sync: writePeerBlocks: %s: ERROR %s", pr.Host, err)
			}
		}
	}
}

// retrievePeerMempool asks the peer for their current copy of their mempool.
func (w *worker) retrievePeerMempool(pr peer.Peer) ([]storage.BlockTx, error) {
	w.evHandler("worker: runPeerUpdatesOperation: retrievePeerMempool: started: %s", pr)
	defer w.evHandler("worker: runPeerUpdatesOperation: retrievePeerMempool: completed: %s", pr)
	
	url := fmt.Sprintf("%s/tx/list", fmt.Sprintf(w.baseURL, pr.Host))
	
	var mempool []storage.BlockTx
	if err := send(http.MethodGet, url, nil, &mempool); err != nil {
		return nil, err
	}
	
	w.evHandler("worker: runPeerUpdatesOperation: retrievePeerMempool: len[%d]", len(mempool))
	
	return mempool, nil
}

// retrievePeerBlocks queries the specified node asking for blocks this
// node does not have, then writes them to disk.
func (w *worker) retrievePeerBlocks(pr peer.Peer) error {
	w.evHandler("worker: sync: retrievePeerBlocks: started: %s", pr)
	defer w.evHandler("worker: sync: retrievePeerBlocks: completed: %s", pr)
	
	from := w.state.RetrieveLatestBlock().Header.Number + 1
	url := fmt.Sprintf("%s/block/list/%d/latest", fmt.Sprintf(w.baseURL, pr.Host), from)
	
	var blocks []storage.Block
	if err := send(http.MethodGet, url, nil, &blocks); err != nil {
		return err
	}
	
	w.evHandler("worker: sync: retrievePeerBlocks: found blocks[%d]", len(blocks))
	
	for _, block := range blocks {
		w.evHandler("worker: sync: retrievePeerBlocks: prevBlk[%s]: newBlk[%s]: numTrans[%d]", block.Header.ParentHash, block.Hash(), len(block.Transactions))
		
		if err := w.state.MinePeerBlock(block); err != nil {
			return err
		}
	}
	
	return nil
}
