package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// peerUpdateInterval represents the interval of time to find new peer
// nodes and update the blockchain on disk with missing blocks.
const peerUpdateInterval = time.Minute

// worker manages the POW workflows for the blockchain.
type worker struct {
	state        *State
	wg           sync.WaitGroup
	ticker       time.Ticker
	shut         chan struct{}
	startMining  chan bool
	cancelMining chan chan struct{}
	txSharing    chan storage.BlockTx
	evHandler    EventHandler
	baseURL      string
}

// runWorker creates a powWorker for starting the POW workflow.
func runWorker(state *State, evHandler EventHandler) {
	// Construct and register this worker to the state. During
	// initialization this worker needs access to the state.
	state.worker = &worker{
		state:        state,
		ticker:       *time.NewTicker(peerUpdateInterval),
		shut:         make(chan struct{}),
		startMining:  make(chan bool, 1),
		cancelMining: make(chan chan struct{}, 1),
		txSharing:    make(chan storage.BlockTx, maxTxShareRequests),
		evHandler:    evHandler,
		baseURL:      "http://%s/v1/node",
	}
	
	// Update this node before starting any support G's.
	state.worker.sync()
	
	// Load the set of operations needed to run.
	operations := []func(){
		state.worker.peerOperations,
		state.worker.miningOperations,
		state.worker.shareTxOperations,
	}
	
	// Set waitgroup to match the number of G's needed
	// for the set of operations we have.
	g := len(operations)
	state.worker.wg.Add(g)
	
	// Don't return until all G's are up and running.
	hasStarted := make(chan bool)
	
	// Start all the operations G's
	for _, op := range operations {
		go func(op func()) {
			defer state.worker.wg.Done()
			hasStarted <- true
			op()
		}(op)
	}
	
	// Wait for the G's to report they are running.
	for i := 0; i < g; i++ {
		<-hasStarted
	}
}

// shutdown terminates the goroutine performing work.
func (w *worker) shutdown() {
	w.evHandler("worker: shutdown: started")
	defer w.evHandler("worker: shutdown: completed")
	
	w.evHandler("worker: shutdown: stop ticker")
	w.ticker.Stop()
	
	w.evHandler("worker: shutdown: signal cancel mining")
	done := w.signalCancelMining()
	done()
	
	w.evHandler("worker: shutdown: terminate goroutines")
	close(w.shut)
	w.wg.Wait()
}

/*// writePeerBlocks queries the specified node asking for
// blocks this node doesn't have and writes them to disk.
func (w *worker) writePeerBlocks(pr peer.Peer) error {
	w.evHandler("worker: runPeerUpdatesOperation: writePeerBlocks: started: %s", pr)
	defer w.evHandler("worker: runPeerUpdatesOperation: writePeerBlocks: completed: %s", pr)

	from := w.state.RetrieveLatestBlock().Header.Number + 1
	url := fmt.Sprintf("%s/block/list/%d/latest", fmt.Sprintf(w.baseURL, pr.Host), from)

	var blocks []storage.Block
	if err := send(http.MethodGet, url, nil, &blocks); err != nil {
		return err
	}

	w.evHandler("worker: runPeerUpdatesOperation: writePeerBlocks: found blocks[%d]", len(blocks))

	for _, block := range blocks {
		w.evHandler("worker: runPeerUpdatesOperation: writePeerBlocks: prevBlk[%s]: newBlk[%s]: numTxs[%d]", block.Header.ParentHash, block.Hash(), len(block.Transactions))

		if err := w.state.MinePeerBlock(block); err != nil {
			return err
		}
	}

	return nil
}*/

// isShutdown is used to test if a shutdown has been signaled.
func (w *worker) isShutdown() bool {
	select {
	case <-w.shut:
		return true
	default:
		return false
	}
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// send is a helper function to send an HTTP request to a node.
func send(method, url string, dataSend any, dataRcv any) error {
	var req *http.Request
	
	switch {
	case dataSend != nil:
		data, err := json.Marshal(dataSend)
		if err != nil {
			return err
		}
		req, err = http.NewRequest(method, url, bytes.NewReader(data))
		if err != nil {
			return err
		}
	
	default:
		var err error
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return err
		}
	}
	
	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	
	if resp.StatusCode != http.StatusOK {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(msg))
	}
	
	if dataRcv != nil {
		if err := json.NewDecoder(resp.Body).Decode(dataRcv); err != nil {
			return err
		}
	}
	
	return nil
}
