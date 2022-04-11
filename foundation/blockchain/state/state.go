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
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
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
	KnownPeers   *peer.Set
	EvHandler    EventHandler
}

// State manages the blockchain database.
type State struct {
	minerAccount storage.Account
	host         string
	dbPath       string
	knownPeers   *peer.Set
	
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
	// Build a safe event handler for use.
	ev := func(v string, args ...any) {
		if cfg.EvHandler != nil {
			cfg.EvHandler(v, args...)
		}
	}
	
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
	blocks, err := strg.ReadAllBlocks(ev)
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
	
	// Create the state to provide suuport for managing the blockchain.
	state := State{
		minerAccount: cfg.MinerAccount,
		host:         cfg.Host,
		dbPath:       cfg.DBPath,
		knownPeers:   cfg.KnownPeers,
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
	
	s.worker.signalShareTransactions(tx)
	
	if n >= s.genesis.TxsPerBlock {
		s.worker.signalStartMining()
	}
	
	return nil
}

// SubmitNodeTransaction accepts a transaction from a node for inclusion.
func (s *State) SubmitNodeTransaction(tx storage.BlockTx) error {
	if err := s.validateTransaction(tx.SignedTx); err != nil {
		return err
	}
	
	n, err := s.mempool.Upsert(tx)
	if err != nil {
		return err
	}
	
	if n >= s.genesis.TxsPerBlock {
		s.worker.signalStartMining()
	}
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// MineNewBlock attempts to create a new block with a proper hash
// that can become the the next block in the chain.
func (s *State) MineNewBlock(ctx context.Context) (storage.Block, time.Duration, error) {
	s.evHandler("state: MineNewBlock: MINING: check mempool count")
	
	// Are there enough transactions in the pool.
	if s.mempool.Count() < s.genesis.TxsPerBlock {
		return storage.Block{}, 0, ErrNotEnoughTransactions
	}
	
	s.evHandler("state: MineNewBlock: MINING: create new block: pick %d", s.genesis.TxsPerBlock)
	
	// Create a new block which owns it's own copy of the transactions.
	txs := s.mempool.PickBest(s.genesis.TxsPerBlock)
	nb := storage.NewBlock(s.minerAccount, s.genesis.Difficulty, s.genesis.TxsPerBlock, s.RetrieveLatestBlock(), txs)
	
	s.evHandler("state: MineNewBlock: MINING: perform POW")
	
	// Attempt to create a new BlockFS by solving the POW puzzle. This can be cancelled.
	blockFS, duration, err := nb.PerformPOW(ctx, s.genesis.Difficulty, s.evHandler)
	if err != nil {
		return storage.Block{}, duration, err
	}
	
	// Just check one more time we were not cancelled.
	if ctx.Err() != nil {
		return storage.Block{}, duration, ctx.Err()
	}
	
	s.evHandler("state: MineNewBlock: MINING: update local state")
	
	if err := s.updateLocalState(blockFS); err != nil {
		return storage.Block{}, duration, err
	}
	
	return blockFS.Block, duration, nil
}

// MinePeerBlock takes a block received from  a peer, validates
// if and if that passes, writes the block to disk.
func (s *State) MinePeerBlock(block storage.Block) error {
	s.evHandler("state: MinePeerBlock: started : block[%s]", block.Hash())
	defer s.evHandler("state: MinePeerBlock: completed")
	
	// If the runMiningOperation function is being executed it needs to stop
	// immediately. The G executing runMiningOperation will not return from the
	// function until done is called. That allows this function to complete
	// its state changes before a new mining operation takes place.
	done := s.worker.signalCancelMining()
	defer func() {
		s.evHandler("state: MinePeerBlock: signal runMiningOperation to terminate")
		done()
	}()
	
	hash, err := block.ValidateBlock(s.latestBlock, s.evHandler)
	if err != nil {
		return err
	}
	
	blockFS := storage.BlockFS{
		Hash:  hash,
		Block: block,
	}
	
	return s.updateLocalState(blockFS)
}

// updateLocalState takes the blockFS and updates the current state of
// the chain, including adding the block to disk.
func (s *State) updateLocalState(blockFS storage.BlockFS) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.evHandler("state: updateLocalState: write to disk")
	
	// Write the new block to the chain on disk.
	if err := s.storage.Write(blockFS); err != nil {
		return err
	}
	s.latestBlock = blockFS.Block
	
	s.evHandler("state: updateLocalState: update accounts and remove from mempool")
	
	// Process the transactions and update the accounts.
	for _, tx := range blockFS.Block.Transactions {
		s.evHandler("state: updateLocalState: tx[%s] update and remove", tx)
		
		// Apply the balance changes based on this transaction.
		if err := s.accounts.ApplyTx(blockFS.Block.Header.MinerAccount, tx); err != nil {
			s.evHandler("state: updateLocalState: WARNING : %s", err)
			continue
		}
		
		// Remove this transaction from the mempool.
		s.mempool.Delete(tx)
	}
	
	s.evHandler("state: updateLocalState: apply mining reward")
	
	// Apply the mining reward for this block.
	s.accounts.ApplyMiningReward(blockFS.Block.Header.MinerAccount)
	
	return nil
}

// validateTransaction takes the signed transaction and validates
// it has a proper signature and other aspects of the data.
func (s *State) validateTransaction(signedTx storage.SignedTx) error {
	if err := signedTx.Validate(); err != nil {
		return err
	}
	
	if err := s.accounts.ValidateNonce(signedTx); err != nil {
		return err
	}
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Truncate resets the chain both on disk and in memory. This
// is used to correct an identified fork.
func (s *State) Truncate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Reset the state of the database.
	// s.mempool.Truncate()
	// s.accounts.Reset()
	// s.latestBlock = storage.Block{}
	// s.storage.Reset()
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RetrieveMempool returns a copy of the mempool.
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

// RetrieveLatestBlock returns a copy the current latest block.
func (s *State) RetrieveLatestBlock() storage.Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return s.latestBlock
}

// RetrieveKnownPeers retrieves a copy of the known peer list.
func (s *State) RetrieveKnownPeers() []peer.Peer {
	return s.knownPeers.Copy(s.host)
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// QueryLatest represents a query to the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

// QueryAccounts returns a copy of the account informatin by account.
func (s *State) QueryAccounts(account storage.Account) map[storage.Account]accounts.Info {
	cpy := s.accounts.Copy()
	
	final := make(map[storage.Account]accounts.Info)
	if info, exists := cpy[account]; exists {
		final[account] = info
	}
	
	return final
}

// QueryMempoolLength returns the current length of the mempool.
func (s *State) QueryMempoolLength() int {
	return s.mempool.Count()
}

// QueryBlocksByNumber returns the set of blocks based on block numbers.
// This function reads the blockchain from the disk first.
func (s *State) QueryBlocksByNumber(from, to uint64) []storage.Block {
	blocks, err := s.storage.ReadAllBlocks(s.evHandler)
	if err != nil {
		return nil
	}
	
	if from == QueryLatest {
		from = blocks[len(blocks)-1].Header.Number
		to = from
	}
	
	var out []storage.Block
	for _, block := range blocks {
		if block.Header.Number >= from && block.Header.Number <= to {
			out = append(out, block)
		}
	}
	
	return out
}

// QueryBlocksByAccount returns the set of blocks by account. If the account
// is empty, all blocks are returns. This function reads the blockchain
// from disk first.
func (s *State) QueryBlocksByAccount(account storage.Account) []storage.Block {
	blocks, err := s.storage.ReadAllBlocks(s.evHandler)
	if err != nil {
		return nil
	}
	
	var out []storage.Block
blocks:
	for _, block := range blocks {
		for _, tx := range block.Transactions {
			from, err := tx.FromAccount()
			if err != nil {
				continue
			}
			if account == "" || from == account || tx.To == account {
				out = append(out, block)
				continue blocks
			}
		}
	}
	
	return out
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// addPeerNode adds a peer to the list of peers.
func (s *State) addPeerNode(peer peer.Peer) error {
	// Don't add this node to the known peer list.
	if peer.Match(s.host) {
		return errors.New("already exists")
	}
	
	s.knownPeers.Add(peer)
	
	return nil
}
