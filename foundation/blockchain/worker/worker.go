// Package worker implements mining, peer updates, and transaction sharing for
// the blockchain.
package worker

import (
	"sync"
	"time"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
)

// peerUpdateInterval represents the interval of time to find new peer
// nodes and update the blockchain on disk with missing blocks.
const peerUpdateInterval = time.Minute

// Worker manages the POW workflows for the blockchain.
type Worker struct {
	state        *state.State
	wg           sync.WaitGroup
	ticker       time.Ticker
	shut         chan struct{}
	startMining  chan bool
	cancelMining chan bool
	txSharing    chan database.BlockTx
	evHandler    state.EventHandler
}

// Run creates a Worker, registers the Worker with the state package, and
// starts up all the background processes.
func Run(state *state.State, evHandler state.EventHandler) {
	// Construct and register this Worker to the state. During
	// initialization this Worker needs access to the state.
	w := Worker{
		state:        state,
		ticker:       *time.NewTicker(peerUpdateInterval),
		shut:         make(chan struct{}),
		startMining:  make(chan bool, 1),
		cancelMining: make(chan bool, 1),
		txSharing:    make(chan database.BlockTx, maxTxShareRequests),
		evHandler:    evHandler,
	}

	// Register this Worker with the state package
	state.Worker = &w

	// Update this node before starting any support G's.
	w.Sync()

	// Load the set of operations needed to run.
	operations := []func(){
		w.peerOperations,
		w.miningOperations,
		w.shareTxOperations,
	}

	// Set waitgroup to match the number of G's needed
	// for the set of operations we have.
	g := len(operations)
	w.wg.Add(g)

	// Don't return until all G's are up and running.
	hasStarted := make(chan bool)

	// Start all the operations G's
	for _, op := range operations {
		go func(op func()) {
			defer w.wg.Done()
			hasStarted <- true
			op()
		}(op)
	}

	// Wait for the G's to report they are running.
	for i := 0; i < g; i++ {
		<-hasStarted
	}
}

// /////////////////////////////////////////////////////////////////
// These methods implements the state.Worker interface.

// Shutdown terminates the goroutine performing work.
func (w *Worker) Shutdown() {
	w.evHandler("Worker: Shutdown: started")
	defer w.evHandler("Worker: Shutdown: completed")

	w.evHandler("Worker: Shutdown: stop ticker")
	w.ticker.Stop()

	w.evHandler("Worker: Shutdown: signal cancel mining")
	w.SignalCancelMining()

	w.evHandler("Worker: Shutdown: terminate goroutines")
	close(w.shut)
	w.wg.Wait()
}

// SignalStartMining starts a mining operation. If there is already a signal
// pending in the channel, just return since a mining operation will start.
func (w *Worker) SignalStartMining() {
	if !w.state.IsMiningAllowed() {
		w.evHandler("state: MinePeerBlock: accepting blocks turned off")
		return
	}

	select {
	case w.startMining <- true:
	default:
	}
	w.evHandler("Worker: SignalStartMining: mining signaled")
}

// SignalCancelMining signals the G executing the runMiningOperation function
// to stop immediately.
func (w *Worker) SignalCancelMining() {
	select {
	case w.cancelMining <- true:
	default:
	}
	w.evHandler("Worker: SignalCancelMining: MINING: CANCEL: signaled")
}

// SignalShareTx queues up a share transaction operation. If
// maxTxShareRequests signals exist in the channel, we won't send these.
func (w *Worker) SignalShareTx(blockTx database.BlockTx) {
	select {
	case w.txSharing <- blockTx:
		w.evHandler("Worker: SignalShareTx: share Tx signaled")
	default:
		w.evHandler("Worker: SignalShareTx: queue full, transactions won't be shared.")
	}
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// isShutdown is used to test if a Shutdown has been signaled.
func (w *Worker) isShutdown() bool {
	select {
	case <-w.shut:
		return true
	default:
		return false
	}
}
