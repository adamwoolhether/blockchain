// Package database maintains account balances and other account information.
package database

import (
	"fmt"
	"sync"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// Info represents information stored in an individual account.
type Info struct {
	Balance uint
	Nonce   uint
}

// Database manages data related to database who have transacted on the blockchain.
type Database struct {
	genesis genesis.Genesis
	records map[storage.Account]Info
	mu      sync.RWMutex
}

// New Constructs a new account and applies genesis and block information.
func New(genesis genesis.Genesis, blocks []storage.Block) *Database {
	db := Database{
		genesis: genesis,
		records: make(map[storage.Account]Info),
	}
	
	for account, balance := range genesis.Balances {
		db.records[account] = Info{Balance: balance}
	}
	
	for _, block := range blocks {
		for _, tx := range block.Transactions.Values() {
			db.ApplyTx(block.Header.MinerAccount, tx)
		}
		db.ApplyMiningReward(block.Header.MinerAccount)
	}
	
	return &db
}

// Reset re-initializes the database back to the genesis information.
func (db *Database) Reset() {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.records = make(map[storage.Account]Info)
	for account, balance := range db.genesis.Balances {
		db.records[account] = Info{Balance: balance}
	}
}

// Remove deletes an database from the database.
func (db *Database) Remove(account storage.Account) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	delete(db.records, account)
}

// CopyRecords makes a copy of the current information for all database
// using value semantics.
func (db *Database) CopyRecords() map[storage.Account]Info {
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	records := make(map[storage.Account]Info)
	for account, info := range db.records {
		records[account] = info
	}
	
	return records
}

// ValidateNonce validates the nonce for the specified transaction is larger
// than the last nonce used by the account who signed the transaction.
func (db *Database) ValidateNonce(tx storage.SignedTx) error {
	from, err := tx.FromAccount()
	if err != nil {
		return err
	}
	
	var info Info
	db.mu.RLock()
	{
		info = db.records[from]
	}
	db.mu.RUnlock()
	
	if tx.Nonce <= info.Nonce {
		return fmt.Errorf("invalid nonce, got %d, exp > %d", tx.Nonce, info.Nonce)
	}
	
	return nil
}

// ApplyMiningReward gives the specified miner account the mining reward.
func (db *Database) ApplyMiningReward(minerAccount storage.Account) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	info := db.records[minerAccount]
	info.Balance += db.genesis.MiningReward
	
	db.records[minerAccount] = info
}

// ApplyTx performs the business logic for applying
// a transaction to the database information.
func (db *Database) ApplyTx(minerAccount storage.Account, tx storage.BlockTx) error {
	from, err := tx.FromAccount()
	if err != nil {
		return fmt.Errorf("invalid signature, %s", err)
	}
	
	db.mu.Lock()
	defer db.mu.Unlock()
	{
		if from == tx.To {
			return fmt.Errorf("invalid transaction, sending money to yourself, from %s to %s", from, tx.To)
		}
		
		fromInfo := db.records[from]
		if tx.Nonce < fromInfo.Nonce {
			return fmt.Errorf("invalid transaction, nonce too small, last %d, tx %d", fromInfo.Nonce, tx.Nonce)
		}
		
		fee := tx.Gas + tx.Tip
		
		if tx.Value+fee > db.records[from].Balance {
			return fmt.Errorf("%s has an insufficient balance", from)
		}
		
		toInfo := db.records[tx.To]
		minerInfo := db.records[minerAccount]
		
		fromInfo.Balance -= tx.Value
		toInfo.Balance += tx.Value
		
		fromInfo.Balance -= fee
		minerInfo.Balance += fee
		
		fromInfo.Nonce = tx.Nonce
		
		db.records[from] = fromInfo
		db.records[tx.To] = toInfo
		db.records[minerAccount] = minerInfo
	}
	return nil
}
