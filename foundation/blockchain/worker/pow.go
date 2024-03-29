package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
)

// CORE NOTE: The POW mining operation is managed by this function which runs
// its own goroutine. When a startMining signal is received (mainly because a
// wallet transaction was received) a block is created and then the POW
// operation starts. This operation can be cancelled if a proposed block is
// received and is validated.

// powOperations handles mining.
func (w *Worker) powOperations() {
	w.evHandler("Worker: powOperations: G started")
	defer w.evHandler("Worker: powOperations: G completed")

	for {
		select {
		case <-w.startMining:
			if !w.isShutdown() {
				w.runPowOperation()
			}
		case <-w.shut:
			w.evHandler("Worker: powOperations: received shut signal")
			return
		}
	}
}

// runPowOperation takes all the transactions from the
// mempool and writes a new block to the database.
func (w *Worker) runPowOperation() {
	w.evHandler("Worker: runMiningOperation: MINING: started")
	defer w.evHandler("Worker: runMiningOperation: MINING: completed")

	// Validate we are allowed to mine and we are not in a resync.
	if !w.state.IsMiningAllowed() {
		w.evHandler("worker: runMiningOperations: MINING: turned off")
		return
	}

	// Make sure there are at least transPerBlock in the mempool.
	length := w.state.MempoolLength()
	if length == 0 {
		w.evHandler("Worker: runMiningOperation: MINING: no transactions to mine: Txs[%d]", length)
		return
	}

	// After running a mining operation, check if a new operation should
	// be signaled again.
	defer func() {
		length := w.state.MempoolLength()
		if length > 0 {
			w.evHandler("Worker: runMiningOperation: MINING: signal new mining operation: Txs[%d]", length)
			w.SignalStartMining()
		}
	}()

	// Drain the cancel mining channel before starting.
	select {
	case <-w.cancelMining:
		w.evHandler("Worker: runMiningOperation: MINING: drained cancel channel")
	default:
	}

	// Create a context so mining can be cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Can't return from this function until these G's are complete.
	var wg sync.WaitGroup
	wg.Add(2)

	// This G exists to cancel the mining operation.
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		select {
		case <-w.cancelMining:
			w.evHandler("Worker: runMiningOperation: MINING: CANCEL: requested")
		case <-ctx.Done():
		}
	}()

	// This G is performing the mining.
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		block, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)

		w.evHandler("Worker: runMiningOperation: MINING: mining duration[%v]", duration)

		if err != nil {
			switch {
			case errors.Is(err, state.ErrNoTransactions):
				w.evHandler("Worker: runMiningOperation: MINING: WARNING: not enough transactions in mempool")
			case ctx.Err() != nil:
				w.evHandler("Worker: runMiningOperation: MINING: CANCEL: complete")
			default:
				w.evHandler("Worker: runMiningOperation: MINING: ERROR: %s", err)
			}
			return
		}

		// WOW, we mined a block. Propose the new block to the network.
		// Log the error, but that's it.
		if err := w.state.NetSendBlockToPeers(block); err != nil {
			w.evHandler("Worker: runMiningOperation: MINING: proposeBlockToPeers: WARNING %s", err)
		}
	}()

	// Wait for both G's to terminate.
	wg.Wait()
}
