package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/state"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// miningOperations handles mining.
func (w *Worker) miningOperations() {
	w.evHandler("Worker: miningOperations: G started")
	defer w.evHandler("Worker: miningOperations: G completed")
	
	for {
		select {
		case <-w.startMining:
			if !w.isShutdown() {
				w.runMiningOperation()
			}
		case <-w.shut:
			w.evHandler("Worker: miningOperations: received shut signal")
			return
		}
	}
}

// runMiningOperation takes all the transactions from the
// mempool and writes a new block to the database.
func (w *Worker) runMiningOperation() {
	w.evHandler("Worker: runMiningOperation: MINING: started")
	defer w.evHandler("Worker: runMiningOperation: MINING: completed")
	
	genesis := w.state.RetrieveGenesis()
	
	// Make sure there are at least transPerBlock in the mempool.
	length := w.state.QueryMempoolLength()
	if length < genesis.TxsPerBlock {
		w.evHandler("Worker: runMiningOperation: MINING: not enough transactions to mine: Txs[%d]", length)
		return
	}
	
	// After running a mining operation, check if a new operation should
	// be signaled again.
	defer func() {
		length := w.state.QueryMempoolLength()
		if length >= genesis.TxsPerBlock {
			w.evHandler("Worker: runMiningOperation: MINING: signal new mining operation: Txs[%d]", length)
			w.SignalStartMining()
		}
	}()
	
	// If mining is signalled to be cancelled by the WriteNextBlock function,
	// this G can't terminate until it is told it can.
	var wait chan struct{}
	defer func() {
		if wait != nil {
			w.evHandler("Worker: runMiningOperation: MINING: termination signal: waiting")
			<-wait
			w.evHandler("Worker: runMiningOperation: MINING: termination signal: received")
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
		case wait = <-w.cancelMining:
			w.evHandler("Worker: runMiningOperation: MINING: cancel mining requested")
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
			case errors.Is(err, state.ErrNotEnoughTransactions):
				w.evHandler("Worker: runMiningOperation: MINING: WARNING: not enough transactions in mempool")
			case ctx.Err() != nil:
				w.evHandler("Worker: runMiningOperation: MINING: CANCELLED: by request")
			default:
				w.evHandler("Worker: runMiningOperation: MINING: ERROR: %s", err)
			}
			return
		}
		
		// WOW, we mined a block. Send the new block to the network.
		// Log the error, but that's it.
		if err := w.sendBlockToPeers(block); err != nil {
			w.evHandler("Worker: runMiningOperation: MINING: sendBlockToPeers: WARNING %s", err)
		}
	}()
	
	// Wait for both G's to terminate.
	wg.Wait()
}

// sendBlockToPeers takes the new mined block and sends it to all know peers.
func (w *Worker) sendBlockToPeers(block storage.Block) error {
	w.evHandler("Worker: runMiningOperation: MINING: sendBlockToPeers: started")
	defer w.evHandler("Worker: runMiningOperation: MINING: sendBlockToPeers: completed")
	
	for _, pr := range w.state.RetrieveKnownPeers() {
		url := fmt.Sprintf("%s/block/next", fmt.Sprintf(w.baseURL, pr.Host))
		
		var status struct {
			Status string        `json:"status"`
			Block  storage.Block `json:"block"`
		}
		
		if err := send(http.MethodPost, url, block, &status); err != nil {
			return fmt.Errorf("%s: %s", pr.Host, err)
		}
		
		w.evHandler("Worker: runMiningOperation: MINING: sendBlockToPeers: sent to peer[%s]", pr)
	}
	
	return nil
}
