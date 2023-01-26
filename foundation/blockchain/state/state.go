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

// Set of different consensus protocols that can be used.
const (
	ConsensusPOW = "POW"
	ConsensusPOA = "POA"
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
	SignalCancelMining()
	SignalShareTx(blockTx database.BlockTx)
}

// /////////////////////////////////////////////////////////////////

// Config represents the configuration requires
// to start the blockchain node.
type Config struct {
	BeneficiaryID  database.AccountID
	Host           string
	Storage        database.Storage
	Genesis        genesis.Genesis
	SelectStrategy string
	KnownPeers     *peer.Set
	EvHandler      EventHandler
	Consensus      string
}

// State manages the blockchain database.
type State struct {
	mu          sync.RWMutex
	resyncWG    sync.WaitGroup
	allowMining bool

	beneficiaryID database.AccountID
	host          string
	evHandler     EventHandler
	consensus     string

	knownPeers *peer.Set
	storage    database.Storage
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

	// Access the storage for the blockchain.
	db, err := database.New(cfg.Genesis, cfg.Storage, ev)
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
		beneficiaryID: cfg.BeneficiaryID,
		host:          cfg.Host,
		storage:       cfg.Storage,
		evHandler:     ev,
		consensus:     cfg.Consensus,
		allowMining:   true,

		knownPeers: cfg.KnownPeers,
		genesis:    cfg.Genesis,
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

// /////////////////////////////////////////////////////////////////

// IsMiningAllowed identifies if we are allowed to mine blocks. This
// might be turned off if the blockchain needs to be re-synced.
func (s *State) IsMiningAllowed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.allowMining
}

// Host returns a copy of host information.
func (s *State) Host() string {
	return s.host
}

// Consensus returns a copy of the consensus algorithm being used.
func (s *State) Consensus() string {
	return s.consensus
}

// Genesis returns a copy of the genesis information.
func (s *State) Genesis() genesis.Genesis {
	return s.genesis
}

// LatestBlock returns a copy the current latest block.
func (s *State) LatestBlock() database.Block {
	return s.db.LatestBlock()
}

// MempoolLength returns the current length of the mempool.
func (s *State) MempoolLength() int {
	return s.mempool.Count()
}

// Mempool returns a copy of the mempool.
func (s *State) Mempool() []database.BlockTx {
	return s.mempool.PickBest()
}

// UpsertMempool adds a new transaction to the mempool.
func (s *State) UpsertMempool(tx database.BlockTx) error {
	return s.mempool.Upsert(tx)
}

// Accounts returns a copy of the database records.
func (s *State) Accounts() map[database.AccountID]database.Account {
	return s.db.Copy()
}

// /////////////////////////////////////////////////////////////////

// AddKnownPeer provides the ability to add
// a new peer to the known peer list.
func (s *State) AddKnownPeer(peer peer.Peer) bool {
	return s.knownPeers.Add(peer)
}

// RemoveKnownPeer provides the ability to remove a
// peer from the known peer list.
func (s *State) RemoveKnownPeer(peer peer.Peer) {
	s.knownPeers.Remove(peer)
}

// KnownExternalPeers retrieves a copy of the known peer list without including this node.
func (s *State) KnownExternalPeers() []peer.Peer {
	return s.knownPeers.Copy(s.host)
}

// KnownPeers retrieves a copy of the full known peer list, including this node.
// Used by the PoAA selection algorithm.
func (s *State) KnownPeers() []peer.Peer {
	return s.knownPeers.Copy("")
}

/*// Truncate resets the chain both on disk and in memory. This
// is used to correct an identified fork.
// DEPRECATED: No longer used
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

// addPeerNode adds a peer to the list of peers.
func (s *State) addPeerNode(peer peer.Peer) error {
	// Don't add this node to the known peer list.
	if peer.Match(s.host) {
		return errors.New("already exists")
	}

	s.knownPeers.Add(peer)

	return nil
}*/
