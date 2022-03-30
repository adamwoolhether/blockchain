// Package state is the core API for the blockchain and implements
// all the business rules and processing.
package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/accounts"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/mempool"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// EventHandler defines a function that is called
// when events occur in the processing of persisting blocks.
type EventHandler func(v string, args ...any)

// Config represents the configuration requires
// to start the blockchain node.
type Config struct {
	MinerAccount string
	Host         string
	DBPath       string
	EvHandler    EventHandler
}

// State manages the blockchain database.
type State struct {
	minerAccount string
	host         string
	dbPath       string
	
	evHandler EventHandler
	
	genesis  genesis.Genesis
	storage  *storage.Storage
	mempool  *mempool.Mempool
	accounts *accounts.Accounts
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
	
	// Create a new accounts value to manage accounts
	// who transact on the blockchain.
	accts := accounts.New(gen)
	
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
		
		genesis:  gen,
		storage:  strg,
		mempool:  mpool,
		accounts: accts,
	}
	
	return &state, nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SubmitWalletTransaction accepts a transaction from a wallet for inclusion.
func (s *State) SubmitWalletTransaction(tx storage.UserTx) error {
	n, err := s.mempool.Upsert(tx)
	if err != nil {
		return err
	}
	
	if n >= s.genesis.TxsPerBlock {
		if err := s.MineNextBlock(); err != nil {
			return err
		}
	}
	
	return nil
}

// MineNextBlock returns a copy of the mempool.
func (s *State) MineNextBlock() error {
	txs := s.mempool.PickBest(2)
	nb := storage.NewBlock(s.minerAccount, s.genesis.Difficulty, s.genesis.TxsPerBlock, txs)
	blockFS := storage.BlockFS{
		Hash:  "my hash",
		Block: nb,
	}
	
	s.evHandler("worker: MineNextBlock: MINING: write block to disk")
	
	// Write new block to the chain on disk.
	if err := s.storage.Write(blockFS); err != nil {
		return err
	}
	
	s.evHandler("worker: MineNextBlock: MINING: remove tx from mempool")
	
	s.accounts.ApplyMiningReward(s.minerAccount)
	
	for _, tx := range txs {
		s.evHandler("worker: MineNextBlock: MINING: UPDATE ACCOUNTS: %s:%d", tx.From, tx.Nonce)
		s.accounts.ApplyTx(s.minerAccount, tx)
		
		s.evHandler("worker: MineNextBlock: MINING: REMOVE: %s:%d", tx.From, tx.Nonce)
		if err := s.mempool.Delete(tx); err != nil {
			return err
		}
	}
	
	// ---------- POW
	// WRITE TO DISK
	// UPDATE ACCOUNTS
	
	// STATE RELOAD
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RetrieveMempool retusn a copy of the mempool.
func (s *State) RetrieveMempool() []storage.UserTx {
	return s.mempool.Copy()
}

// RetrieveGenesis returns a copy of the genesis information.
func (s *State) RetrieveGenesis() genesis.Genesis {
	return s.genesis
}

// RetrieveAccounts returns a copy of the set of account information.
func (s *State) RetrieveAccounts() map[string]accounts.Info {
	return s.accounts.Copy()
}
