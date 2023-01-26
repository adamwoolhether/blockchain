package worker

import (
	"context"
	"errors"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
)

// CORE NOTE: PoA mining operations are managed by this function which runs
// its own goroutine. The node starts a loop that is on a 12-second timer.
// At the beginning of each cycle the selection algorithm is executed,
// determining if this node needs to mine the next block. If this node
// isn't selected, it waits for the next cycle to check the selection algorithm again.

// cycleDuration sets the mining operation to happen every 12 seconds.
const secondsPerCycle = 12
const cycleDuration = secondsPerCycle * time.Second

// poaOperations handles mining
func (w *Worker) poaOperations() {
	w.evHandler("worker: poaOperations: G started")
	defer w.evHandler("worker: poaOperations: G completed")

	ticker := time.NewTicker(cycleDuration)

	// Start this on a secondsPerCycle mark: ex. MM.00, MM.12, MM.24, MM.36
	resetTicker(ticker, secondsPerCycle*time.Second)

	for {
		select {
		case <-ticker.C:
			if !w.isShutdown() {
				w.runPoaOperation()
			}
		case <-w.shut:
			w.evHandler("worker: poaOperations: received shut down signal")
			return
		}

		// Reset the ticker for the next cycle.
		resetTicker(ticker, 0)
	}
}

// runPoaOperation takes all transactions from the mempool and writes a new block to the database.
func (w *Worker) runPoaOperation() {
	w.evHandler("worker: runPoaOperations: started")
	defer w.evHandler("worker: runPoaOperations: completed")

	// Run the selection algorithm.
	peer := w.selection()
	w.evHandler("worker: runPoaOperations: SELECTED: %s", peer)

	// If we aren't selected, return and wait for new block.
	if peer != w.state.Host() {
		return
	}

	// Validate we are allowed to mine and aren't in a resync.
	if !w.state.IsMiningAllowed() {
		w.evHandler("worker: runPoaOperations: MINING: turned off")
		return
	}

	// Ensure there are transactions in the mempool.
	length := w.state.MempoolLength()
	if length == 0 {
		w.evHandler("worker: runPoaOperations: MINING: no transactions to mine: Tx[%d]", length)
		return
	}

	// Drain the cancel mining channel before starting.
	select {
	case <-w.cancelMining:
		w.evHandler("worker: runPoaOperations: MINING: drained cancel channel")
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
			w.evHandler("worker: runPoaOperations: MINING: CANCEL: requested")
		case <-ctx.Done():
		}
	}()

	// This G is performing the mining
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		block, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)

		w.evHandler("worker: runPoaOperations: MINING: mining duration[%v]", duration)

		if err != nil {
			switch {
			case errors.Is(err, state.ErrNoTransactions):
				w.evHandler("worker: runPoaOperations: MINING: WARNING: no transactions in mempool")
			case ctx.Err() != nil:
				w.evHandler("worker: runPoaOperations: MINING: CANCEL: completed")
			default:
				w.evHandler("worker: runPoaOperations: MINING: ERROR: %s", err)
			}
			return
		}

		// The block is mined. Propose the new block to the network.
		// Log the error if present.
		if err := w.state.NetSendBlockToPeers(block); err != nil {
			w.evHandler("worker: runPoaOperations: MINING: proposeBlockToPeers: WARNING: %s", err)
		}
	}()

	// Wait for both G's to terminate:
	wg.Wait()
}

// selection selects a peer to be the next one to mine a block.
func (w *Worker) selection() string {
	// Retrieve known peers list, including this node.
	peers := w.state.KnownPeers()

	// Log information for clarity about the list
	w.evHandler("worker: runPoaOperation: selection: Host %s, List %v", w.state.Host(), peers)

	// Sort current list of peers by host.
	names := make([]string, len(peers))
	for i, peer := range peers {
		names[i] = peer.Host
	}
	sort.Strings(names)

	// Based on the latest block, pick an index number from the registry.
	h := fnv.New32a()
	h.Write([]byte(w.state.LatestBlock().Hash()))
	integerHash := h.Sum32()
	i := integerHash % uint32(len(names))

	// Return the selected node's name
	return names[i]
}

// /////////////////////////////////////////////////////////////////

// resetTicker ensures that the next tick happens on the described candence.
func resetTicker(ticker *time.Ticker, waitOnSecond time.Duration) {
	nextTick := time.Now().Add(cycleDuration).Round(waitOnSecond)
	diff := time.Until(nextTick)
	ticker.Reset(diff)
}
