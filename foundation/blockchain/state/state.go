// Package state is the core API for the blockchain and implements
// all the business rules and processing.
package state

import (
	"sync"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/accounts"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/mempool"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// EventHandler defines a function that is called
// when events occur in the processing of persisting blocks.
type EventHandler func(v string, args ...any)

// Worker interface represents the behavior required to be implemented
// by any package providing background blockchain processes support.
type Worker interface {
	Shutdown()
	SignalStartMining()
	SignalCancelMining() (done func())
	SignalShareTx(blockTx storage.BlockTx)
}

// /////////////////////////////////////////////////////////////////

// Config represents the configuration requires
// to start the blockchain node.
type Config struct {
	MinerAccount   storage.Account
	Host           string
	DBPath         string
	SelectStrategy string
	KnownPeers     *peer.Set
	EvHandler      EventHandler
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
	
	Worker Worker
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
	blocks, err := strg.ReadAllBlocks(ev, true)
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
	accts := accounts.New(gen, blocks)
	
	// Construct a mempool with the specified sort strategy.
	mpool, err := mempool.NewWithStrategy(cfg.SelectStrategy)
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
	
	// The Worker is not set here. The call to worker.Run will assign
	// itself and start everything up and running for the node.
	
	return &state, nil
}

// Shutdown cleanly brings the node down.
func (s *State) Shutdown() error {
	// Make sure the database fiel is properly closed.
	defer func() {
		s.storage.Close()
	}()
	
	// Stop all blockchain writing activity.
	s.Worker.Shutdown()
	
	return nil
}

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

// // addPeerNode adds a peer to the list of peers.
// func (s *State) addPeerNode(peer peer.Peer) error {
// 	// Don't add this node to the known peer list.
// 	if peer.Match(s.host) {
// 		return errors.New("already exists")
// 	}
//
// 	s.knownPeers.Add(peer)
//
// 	return nil
// }
