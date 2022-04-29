// Package state is the core API for the blockchain and implements
// all the business rules and processing.
package state

import (
	"sync"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/mempool"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
)

// EventHandler defines a function that is called
// when events occur in the processing of persisting blocks.
type EventHandler func(v string, args ...any)

// Worker interface represents the behavior required to be implemented
// by any package providing support for mining, peer updates, and tx sharing.
type Worker interface {
	Shutdown()
	Sync()
	SignalStartMining()
	SignalCancelMining() (done func())
	SignalShareTx(blockTx database.BlockTx)
}

// /////////////////////////////////////////////////////////////////

// Config represents the configuration requires
// to start the blockchain node.
type Config struct {
	MinerAccountID database.AccountID
	Host           string
	DBPath         string
	SelectStrategy string
	KnownPeers     *peer.Set
	EvHandler      EventHandler
}

// State manages the blockchain database.
type State struct {
	mu sync.RWMutex
	
	minerAccountID database.AccountID
	host           string
	dbPath         string
	evHandler      EventHandler
	
	allowMining bool
	resyncWG    sync.WaitGroup
	
	knownPeers *peer.Set
	genesis    genesis.Genesis
	mempool    *mempool.Mempool
	db         *database.Database
	
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
	db, err := database.New(cfg.DBPath, gen, ev)
	if err != nil {
		return nil, err
	}
	
	// Construct a mempool with the specified sort strategy.
	mpool, err := mempool.NewWithStrategy(cfg.SelectStrategy)
	if err != nil {
		return nil, err
	}
	
	// Create the state to provide suuport for managing the blockchain.
	state := State{
		minerAccountID: cfg.MinerAccountID,
		host:           cfg.Host,
		dbPath:         cfg.DBPath,
		evHandler:      ev,
		allowMining:    true,
		
		knownPeers: cfg.KnownPeers,
		genesis:    gen,
		mempool:    mpool,
		db:         db,
	}
	
	// The Worker is not set here. The call to worker.Run will assign
	// itself and start everything up and running for the node.
	
	return &state, nil
}

// Shutdown cleanly brings the node down.
func (s *State) Shutdown() error {
	s.evHandler("state: shutdown: started")
	defer s.evHandler("state: shutdown: completed")
	
	// Make sure the database field is properly closed.
	defer func() {
		s.db.Close()
	}()
	
	// Stop all blockchain writing activity.
	s.Worker.Shutdown()
	
	// Wait for resync to finish.
	s.resyncWG.Wait()
	
	return nil
}

// IsMiningAllowed identifies if we are allowed to mine blocks. This
// might be turned off if the blockchain needs to be re-synced.
func (s *State) IsMiningAllowed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.allowMining
}

// TurnMiningOn sets the allowMining flag back to true.
func (s *State) TurnMiningOn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.allowMining = true
}

// Resync resets the chain both on disk and in memory. This is used to
// correct an identified fork. No mining is allowed to take place while this
// process is running. New transactions can be placed in the mempool.
func (s *State) Resync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Don't allow mining to continue.
	s.allowMining = false
	
	// Reset the state of the blockchain node.
	s.db.Reset()
	
	// Resync the state of the blockchain.
	s.resyncWG.Add(1)
	go func() {
		s.evHandler("state: Resync: started: ***********************")
		defer func() {
			s.TurnMiningOn()
			s.evHandler("state: Resync: completed: ***********************")
			s.resyncWG.Done()
		}()
		
		s.Worker.Sync()
	}()
	
	return nil
}

// Truncate resets the chain both on disk and in memory. This
// is used to correct an identified fork.
func (s *State) Truncate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Reset the state of the database.
	// s.mempool.Truncate()
	// s.database.Reset()
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
