package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/peer"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// AddKnownPeer provides the ability to add a new peer.
func (s *State) AddKnownPeer(peer peer.Peer) {
	s.knownPeers.Add(peer)
}

// UpsertMempool adds a new transaction to the mempool.
func (s *State) UpsertMempool(tx storage.BlockTx) (int, error) {
	return s.mempool.Upsert(tx)
}
