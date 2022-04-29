// Package mempool maintains the mempool for the blockchain.
package mempool

import (
	"fmt"
	"strings"
	"sync"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/mempool/selector"
)

// Mempool represents a cache of transactions organized by account:nonce.
type Mempool struct {
	mu       sync.RWMutex
	pool     map[string]database.BlockTx
	selectFn selector.Func
}

// New constructs a new mempool with the specified sort strategy.
func New() (*Mempool, error) {
	return NewWithStrategy(selector.StrategyTip)
}

// NewWithStrategy  constructs a new mempool with the specified sort strategy.
func NewWithStrategy(strategy string) (*Mempool, error) {
	selectFn, err := selector.Retrieve(strategy)
	if err != nil {
		return nil, err
	}
	
	mp := Mempool{
		pool:     make(map[string]database.BlockTx),
		selectFn: selectFn,
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
func (mp *Mempool) Upsert(tx database.BlockTx) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	key, err := mapKey(tx)
	if err != nil {
		return nil
	}
	
	mp.pool[key] = tx
	
	return nil
}

// Delete removes a transaction from the mempool.
func (mp *Mempool) Delete(tx database.BlockTx) error {
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
func (mp *Mempool) Copy() []database.BlockTx {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	cpy := []database.BlockTx{}
	for _, tx := range mp.pool {
		cpy = append(cpy, tx)
	}
	
	return cpy
}

// PickBest uses the configured sort strategy to return the next
// set of transactions for the next bock.
func (mp *Mempool) PickBest(howMany ...int) []database.BlockTx {
	number := -1
	if len(howMany) > 0 {
		number = howMany[0]
	}
	
	// Copy all the transactions for each account
	// into separate slices for each account.
	m := make(map[database.AccountID][]database.BlockTx)
	mp.mu.RLock()
	{
		if number == -1 {
			number = len(mp.pool)
		}
		
		for key, tx := range mp.pool {
			account := accountFromMapKey(key)
			m[account] = append(m[account], tx)
		}
	}
	mp.mu.RUnlock()
	
	return mp.selectFn(m, number)
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func mapKey(tx database.BlockTx) (string, error) {
	account, err := tx.FromAccount()
	if err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%s:%d", account, tx.Nonce), nil
}

// accountFromMapKey extracts the account information from mapkey.
func accountFromMapKey(key string) database.AccountID {
	return database.AccountID(strings.Split(key, ":")[0])
}
