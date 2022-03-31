// Package mempool maintains the mempool for the blockchain.
package mempool

import (
	"fmt"
	"sync"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// Mempool represents a cache of transactions organized by account:nonce.
type Mempool struct {
	pool map[string]storage.BlockTx
	mu   sync.RWMutex
}

// New constructs a new mempool with the specified sort strategy.
func New() (*Mempool, error) {
	mp := Mempool{
		pool: make(map[string]storage.BlockTx),
	}
	
	return &mp, nil
}

// Count return the current number of transaction in the pool.
func (mp *Mempool) Count() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	return len(mp.pool)
}

// Upsert adds or replaces a transaction from the mempool.
func (mp *Mempool) Upsert(tx storage.BlockTx) (int, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	key, err := mapKey(tx)
	if err != nil {
		return 0, nil
	}
	
	mp.pool[key] = tx
	
	return len(mp.pool), nil
}

// Delete removes a transaction from the mempool.
func (mp *Mempool) Delete(tx storage.BlockTx) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	key, err := mapKey(tx)
	if err != nil {
		return err
	}
	
	delete(mp.pool, key)
	
	return nil
}

// Copy uses the configured sort strategy to return the next
// set of transactions for the next bock.
func (mp *Mempool) Copy() []storage.BlockTx {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	cpy := []storage.BlockTx{}
	for _, tx := range mp.pool {
		cpy = append(cpy, tx)
	}
	
	return cpy
}

// PickBest uses the configured sort strategy to return the next
// set of transactions for the next bock.
func (mp *Mempool) PickBest(howMany int) []storage.BlockTx {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	cpy := []storage.BlockTx{}
	for _, tx := range mp.pool {
		cpy = append(cpy, tx)
		if len(cpy) == howMany {
			break
		}
	}
	
	return cpy
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func mapKey(tx storage.BlockTx) (string, error) {
	account, err := tx.FromAccount()
	if err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%s:%d", account, tx.Nonce), nil
}
