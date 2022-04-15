package worker

import (
	"fmt"
	"net/http"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// sync updates the peer list, mempool, and blocks.
func (w *Worker) sync() {
	w.evHandler("Worker: sync: started")
	defer w.evHandler("Worker: sync: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		// Retrieve the status of this peer.
		peerStatus, err := w.queryPeerStatus(pr)
		if err != nil {
			w.evHandler("Worker: sync: queryPeerStatus: %s: ERROR: %s", pr.Host, err)
		}
		
		// Add new peers to this nodes list.
		w.addNewPeers(peerStatus.KnownPeers)
		
		// Update the mempool.
		pool, err := w.retrievePeerMempool(pr)
		if err != nil {
			w.evHandler("Worker: sync: retrievePeerMempool: %s: ERROR: %s", pr.Host, err)
		}
		for _, tx := range pool {
			w.evHandler("Worker: sync: retrievePeerMempool: %s: Add Tx: %s", pr.Host, tx.SignatureString()[:16])
			w.state.UpsertMempool(tx)
		}
		
		// If this peer has blocks we don't have, we need to add them.
		if peerStatus.LatestBlockNumber > w.state.RetrieveLatestBlock().Header.Number {
			w.evHandler("Worker: sync: writePeerBlocks: %s: latestBlockNumber[%d]", pr.Host, peerStatus.LatestBlockNumber)
			
			if err := w.retrievePeerBlocks(pr); err != nil {
				w.evHandler("Worker: sync: writePeerBlocks: %s: ERROR %s", pr.Host, err)
			}
		}
	}
}

// retrievePeerMempool asks the peer for their current copy of their mempool.
func (w *Worker) retrievePeerMempool(pr peer.Peer) ([]storage.BlockTx, error) {
	w.evHandler("Worker: runPeerUpdatesOperation: retrievePeerMempool: started: %s", pr)
	defer w.evHandler("Worker: runPeerUpdatesOperation: retrievePeerMempool: completed: %s", pr)
	
	url := fmt.Sprintf("%s/tx/list", fmt.Sprintf(w.baseURL, pr.Host))
	
	var mempool []storage.BlockTx
	if err := send(http.MethodGet, url, nil, &mempool); err != nil {
		return nil, err
	}
	
	w.evHandler("Worker: runPeerUpdatesOperation: retrievePeerMempool: len[%d]", len(mempool))
	
	return mempool, nil
}

// retrievePeerBlocks queries the specified node asking for blocks this
// node does not have, then writes them to disk.
func (w *Worker) retrievePeerBlocks(pr peer.Peer) error {
	w.evHandler("Worker: sync: retrievePeerBlocks: started: %s", pr)
	defer w.evHandler("Worker: sync: retrievePeerBlocks: completed: %s", pr)
	
	from := w.state.RetrieveLatestBlock().Header.Number + 1
	url := fmt.Sprintf("%s/block/list/%d/latest", fmt.Sprintf(w.baseURL, pr.Host), from)
	
	var blocksFS []storage.BlockFS
	if err := send(http.MethodGet, url, nil, &blocksFS); err != nil {
		return err
	}
	
	w.evHandler("Worker: sync: retrievePeerBlocks: found blocks[%d]", len(blocksFS))
	
	for _, blockFS := range blocksFS {
		block, err := storage.ToBlock(blockFS)
		if err != nil {
			return err
		}
		
		w.evHandler("Worker: sync: retrievePeerBlocks: prevBlk[%s]: newBlk[%s]: numTrans[%d]", block.Header.ParentHash, block.Hash(), len(block.Transactions.Values()))
		
		if err := w.state.MinePeerBlock(block); err != nil {
			return err
		}
	}
	
	return nil
}
