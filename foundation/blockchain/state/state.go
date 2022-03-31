// Package state is the core API for the blockchain and implements
// all the business rules and processing.
package state

import (
	"context"
	"errors"
	"sync"
	"time"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/accounts"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/mempool"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// ErrNotEnoughTransactions is returned when a block is requested
// to be created and there aren't enough transactions.
var ErrNotEnoughTransactions = errors.New("not enough transactions in mempool")

// EventHandler defines a function that is called
// when events occur in the processing of persisting blocks.
type EventHandler func(v string, args ...any)

// Config represents the configuration requires
// to start the blockchain node.
type Config struct {
	MinerAccount storage.Account
	Host         string
	DBPath       string
	EvHandler    EventHandler
}

// State manages the blockchain database.
type State struct {
	minerAccount storage.Account
	host         string
	dbPath       string
	
	evHandler EventHandler
	
	genesis     genesis.Genesis
	storage     *storage.Storage
	mempool     *mempool.Mempool
	accounts    *accounts.Accounts
	latestBlock storage.Block
	mu          sync.Mutex
	
	worker *worker
}

// New constructs a new blockchain for data management.
func New(cfg Config) (*State, error) {
	// Load the genesis file to get starting
	// balances for founders of the blockchain.
	gen, err := genesis.Load()
	if err != nil {
		return nil, err
	}
	
	// Access the storage for the blockchain.
	strg, err := storage.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	
	// Load all existing blocks from storage into memory for processing.
	// This won't work in a large system like Ethereum!
	blocks, err := strg.ReadAllBlocks()
	if err != nil {
		return nil, err
	}
	
	// Keep the latest blocks from the blockchain.
	var latestBlock storage.Block
	if len(blocks) > 0 {
		latestBlock = blocks[len(blocks)-1]
	}
	
	// Create a new accounts value to manage accounts
	// who transact on the blockchain.
	accts := accounts.New(gen)
	
	// Process the blocks and transactions for each account.
	for _, block := range blocks {
		for _, tx := range block.Transactions {
			// Apply the balance changes based for this transaction.
			if err := accts.ApplyTx(block.Header.MinerAccount, tx); err != nil {
				return nil, err
			}
		}
		
		// Apply the mining reward for this block
		accts.ApplyMiningReward(block.Header.MinerAccount)
	}
	
	// Construct a mempool with the specified sort strategy.
	mpool, err := mempool.New()
	if err != nil {
		return nil, err
	}
	
	// Build a safe event handler for use.
	ev := func(v string, args ...any) {
		if cfg.EvHandler != nil {
			cfg.EvHandler(v, args...)
		}
	}
	
	// Create the state to provide suuport for managing the blockchain.
	state := State{
		minerAccount: cfg.MinerAccount,
		host:         cfg.Host,
		dbPath:       cfg.DBPath,
		evHandler:    ev,
		
		genesis:     gen,
		storage:     strg,
		mempool:     mpool,
		accounts:    accts,
		latestBlock: latestBlock,
	}
	
	// Run the worker which will assign itself to this state.
	runWorker(&state, cfg.EvHandler)
	
	return &state, nil
}

// Shutdown cleanly brings the node down.
func (s *State) Shutdown() error {
	// Make sure the database fiel is properly closed.
	defer func() {
		s.storage.Close()
	}()
	
	// Stop all blockchain writing activity.
	s.worker.shutdown()
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SubmitWalletTransaction accepts a transaction from a wallet for inclusion.
func (s *State) SubmitWalletTransaction(signedTx storage.SignedTx) error {
	if err := s.validateTransaction(signedTx); err != nil {
		return err
	}
	
	tx := storage.NewBlockTx(signedTx, s.genesis.GasPrice)
	
	n, err := s.mempool.Upsert(tx)
	if err != nil {
		return err
	}
	
	if n >= s.genesis.TxsPerBlock {
		s.worker.signalStartMining()
	}
	
	return nil
}

// MineNewBlock writes the published transaction for the mempool to disk.
func (s *State) MineNewBlock(ctx context.Context) (storage.Block, time.Duration, error) {
	s.evHandler("worker: MineNewBlock: MINING: check mempool count")
	
	// Are there enough transactions in the pool.
	if s.mempool.Count() < s.genesis.TxsPerBlock {
		return storage.Block{}, 0, ErrNotEnoughTransactions
	}
	
	s.evHandler("worker: MineNewBlock: MINING: create new block: pick %d", s.genesis.TxsPerBlock)
	
	txs := s.mempool.PickBest(2)
	block := storage.NewBlock(s.minerAccount, s.genesis.Difficulty, s.genesis.TxsPerBlock, s.RetrieveLatestBlock(), txs)
	
	s.evHandler("worker: MineNewBlock: MINING: copy accounts and update")
	
	// Process the transactions against a copy of the accounts.
	accts := s.accounts.Clone()
	for _, tx := range block.Transactions {
		// Apply the balance changes based on this transaction.
		if err := accts.ApplyTx(s.minerAccount, tx); err != nil {
			s.evHandler("worker: MineNewBlock: MINING: WARNING: %s", err)
			continue
		}
		
		// Update the total gas and tip fees.
		block.Header.TotalGas += tx.Gas
		block.Header.TotalTip += tx.Tip
	}
	
	// Apply the mining reward for this block.
	accts.ApplyMiningReward(s.minerAccount)
	
	s.evHandler("worker: MineNewBlock: MINING: perform POW")
	
	// Attempt to create a new BlockFS by solving
	// the POW puzzle. This can be cancelled.
	blockFS, duration, err := performPOW(ctx, s.genesis.Difficulty, block, s.evHandler)
	if err != nil {
		return storage.Block{}, duration, err
	}
	
	// Check one more time cancellation hasn't occurred.
	if ctx.Err() != nil {
		return storage.Block{}, duration, ctx.Err()
	}
	
	// Ensure the following state changes are done atomically.
	s.mu.Lock()
	defer s.mu.Unlock()
	{
		s.evHandler("worker: MineNewBlock: MINING: write block to disk")
		
		// Write new block to the chain on disk.
		if err := s.storage.Write(blockFS); err != nil {
			return storage.Block{}, duration, err
		}
		
		s.evHandler("worker: MineNewBlock: MINING: apply new account updates")
		
		s.accounts.Replace(accts)
		s.latestBlock = blockFS.Block
		
		// Remove the transactions from this block.
		for _, tx := range block.Transactions {
			s.evHandler("worker: MineNewBlock: MINING: remove tx from mempool: tx[%s]", tx)
			s.mempool.Delete(tx)
		}
	}
	
	return blockFS.Block, duration, nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RetrieveMempool retusn a copy of the mempool.
func (s *State) RetrieveMempool() []storage.BlockTx {
	return s.mempool.Copy()
}

// RetrieveGenesis returns a copy of the genesis information.
func (s *State) RetrieveGenesis() genesis.Genesis {
	return s.genesis
}

// RetrieveAccounts returns a copy of the set of account information.
func (s *State) RetrieveAccounts() map[storage.Account]accounts.Info {
	return s.accounts.Copy()
}

// RetrieveLatestBlock returns a copy of the current latest block.
func (s *State) RetrieveLatestBlock() storage.Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.latestBlock
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// QueryMempoolLength returns the current length of the mempool.
func (s *State) QueryMempoolLength() int {
	return s.mempool.Count()
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// validateTransaction takes the signed transaction and validates
// it has a proper signature and other aspects of the data.
func (s *State) validateTransaction(signedTx storage.SignedTx) error {
	if err := signedTx.Validate(); err != nil {
		return err
	}
	
	return nil
}

// // MineNextBlock returns a copy of the mempool.
// func (s *State) MineNextBlock() error {
// 	txs := s.mempool.PickBest(2)
// 	block := storage.NewBlock(s.minerAccount, s.genesis.Difficulty, s.genesis.TxsPerBlock, s.latestBlock, txs)
//
// 	s.evHandler("worker: MineNextBlock: MINING: find hash")
//
// 	blockFS, _, err := performPOW(context.TODO(), s.genesis.Difficulty, block, s.evHandler)
// 	if err != nil {
// 		return err
// 	}
//
// 	s.evHandler("worker: MineNextBlock: MINING: write block to disk")
//
// 	// Write new block to the chain on disk.
// 	if err := s.storage.Write(blockFS); err != nil {
// 		return err
// 	}
//
// 	s.evHandler("worker: MineNextBlock: MINING: remove tx from mempool")
//
// 	s.accounts.ApplyMiningReward(s.minerAccount)
//
// 	for _, tx := range txs {
// 		from, err := tx.FromAccount() // TODO
// 		if err != nil {
// 			return err
// 		}
//
// 		s.evHandler("worker: MineNextBlock: MINING: UPDATE ACCOUNTS: %s:%d", from, tx.Nonce)
// 		s.accounts.ApplyTx(s.minerAccount, tx)
//
// 		s.evHandler("worker: MineNextBlock: MINING: REMOVE: %s:%d", from, tx.Nonce)
// 		if err := s.mempool.Delete(tx); err != nil {
// 			return err
// 		}
// 	}
//
// 	// Save this is the latest block.
// 	s.latestBlock = blockFS.Block
//
// 	return nil
// }
