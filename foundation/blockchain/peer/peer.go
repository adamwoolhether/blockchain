package peer

import "sync"

// Peer represents information about a State in the network.
type Peer struct {
	Host string
}

// New constructs a new info value.
func New(host string) Peer {
	return Peer{
		Host: host,
	}
}

// Match validates if the specified host matches this node.
func (p Peer) Match(host string) bool {
	return p.Host == host
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Status represents information about
// the status of any given peer.
type Status struct {
	LatestBlockHash   string `json:"latest_block_hash"`
	LatestBlockNumber uint64 `json:"latest_block_number"`
	KnownPeers        []Peer `json:"known_peers"`
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Set represents the data representation to maintain a set of know peers.
type Set struct {
	mu  sync.RWMutex
	set map[Peer]struct{}
}

// NewSet constructs a new info set to manage node peer information.
func NewSet() *Set {
	return &Set{
		set: make(map[Peer]struct{}),
	}
}

// Add adds a new node to the set.
func (s *Set) Add(peer Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.set[peer]
	if !exists {
		s.set[peer] = struct{}{}
		return true
	}

	return false
}

// Remove removes a node from the set.
func (s *Set) Remove(peer Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.set, peer)
}

// Copy returns a list of known peers.
func (s *Set) Copy(host string) []Peer {
	s.mu.Lock()
	defer s.mu.Unlock()

	var peers []Peer
	for peer := range s.set {
		if !peer.Match(host) {
			peers = append(peers, peer)
		}
	}

	return peers
}
