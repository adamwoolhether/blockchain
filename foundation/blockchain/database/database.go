// Package database maintains account balances and other account information.
package database

import (
	"fmt"
	"sort"
	"sync"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/signature"
)

// Storage interface represents the behavior required to be implemented by any
// package providing support for storing and reading the blockchain from the blockchain.
type Storage interface {
	Write(blockData BlockData) error
	GetBlock(num uint64) (BlockData, error)
	ForEach() Iterator
	Close() error
	Reset() error
}

// Iterator interface represents the behavior required to be implemented by any
// package providing support to iterate over the blocks.
type Iterator interface {
	Next() (BlockData, error)
	Done() bool
}

// /////////////////////////////////////////////////////////////////

// Database manages data related to database who have transacted on the blockchain.
type Database struct {
	mu sync.RWMutex

	genesis     genesis.Genesis
	latestBlock Block
	accounts    map[AccountID]Account

	storage Storage
}

// New Constructs a new account, applies genesis and block information
// and reads/writes the blockchain database on disk if a dbPath is provided.
// New constructs a new database and applies account genesis information and
// reads/writes the blockchain database on disk if a dbPath is provided.
// New constructs a new database and applies account genesis information and
// reads/writes the blockchain database on disk if a dbPath is provided.
func New(genesis genesis.Genesis, storage Storage, evHandler func(v string, args ...any)) (*Database, error) {
	db := Database{
		genesis:  genesis,
		accounts: make(map[AccountID]Account),
		storage:  storage,
	}

	// Update the database with account balance information from genesis.
	for accountStr, balance := range genesis.Balances {
		accountID, err := ToAccountID(accountStr)
		if err != nil {
			return nil, err
		}
		db.accounts[accountID] = newAccount(accountID, balance)
	}

	// Read all the blocks from storage.
	iter := db.ForEach()
	for block, err := iter.Next(); !iter.Done(); block, err = iter.Next() {
		if err != nil {
			return nil, err
		}

		// Validate the block values and cryptographic audit trail.
		if err := block.ValidateBlock(db.latestBlock, db.HashState(), evHandler); err != nil {
			return nil, err
		}

		// Update the database with account balance information from genesis.
		for accountStr, balance := range genesis.Balances {
			accountID, err := ToAccountID(accountStr)
			if err != nil {
				return nil, err
			}
			db.accounts[accountID] = newAccount(accountID, balance)
		}

		// Update the database with the transaction information.
		for _, tx := range block.Transactions.Values() {
			db.ApplyTx(block, tx)
		}
		db.ApplyMiningReward(block)

		// Update the current latest block.
		db.latestBlock = block
	}

	return &db, nil
}

// Close closes the open blocks database.
func (db *Database) Close() {
	db.storage.Close()
}

// Reset re-initializes the database back to the genesis state.
func (db *Database) Reset() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.storage.Reset()

	// Initialize the database block to the genesis information.
	db.latestBlock = Block{}
	db.accounts = make(map[AccountID]Account)
	for accountStr, balance := range db.genesis.Balances {
		accountID, err := ToAccountID(accountStr)
		if err != nil {
			return err
		}

		db.accounts[accountID] = newAccount(accountID, balance)
	}

	return nil
}

// Remove deletes an account from the database.
func (db *Database) Remove(accountID AccountID) {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.accounts, accountID)
}

// CopyAccounts makes a copy of the current database for all account.
func (db *Database) CopyAccounts() map[AccountID]Account {
	db.mu.RLock()
	defer db.mu.RUnlock()

	accounts := make(map[AccountID]Account)
	for accountID, info := range db.accounts {
		accounts[accountID] = info
	}

	return accounts
}

// HashState returns a hash based on the contents of the accounts and
// their balances. This is added to each block and checked by peers.
func (db *Database) HashState() string {
	accounts := make([]Account, 0, len(db.accounts))
	db.mu.RLock()
	{
		for _, account := range db.accounts {
			accounts = append(accounts, account)
		}
	}
	db.mu.RUnlock()

	sort.Sort(byAccount(accounts))

	return signature.Hash(accounts)
}

// ApplyMiningReward gives the specified miner account the mining reward.
func (db *Database) ApplyMiningReward(block Block) {
	db.mu.Lock()
	defer db.mu.Unlock()

	account := db.accounts[block.Header.BeneficiaryID]
	account.Balance += db.genesis.MiningReward

	db.accounts[block.Header.BeneficiaryID] = account
}

// ApplyTx performs the business logic for applying
// a transaction to the database information.
func (db *Database) ApplyTx(block Block, tx BlockTx) error {

	// Capture the from address from the signature of the transaction.
	fromID, err := tx.FromAccount()
	if err != nil {
		return fmt.Errorf("invalid signature, %s", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	{
		// Capture accounts from the database.
		from, exists := db.accounts[fromID]
		if !exists {
			from = newAccount(fromID, 0)
		}

		to, exists := db.accounts[tx.ToID]
		if !exists {
			to = newAccount(tx.ToID, 0)
		}

		bnfc, exists := db.accounts[block.Header.BeneficiaryID]
		if !exists {
			bnfc = newAccount(block.Header.BeneficiaryID, 0)
		}

		// The account needs to pay the gas fee regardless. Take the
		// remaining balance if the account doesn't hold enough for
		// the full amount of gas. This helps prevent bad actors.
		gasFee := tx.GasPrice * tx.GasUnits
		if gasFee > from.Balance {
			gasFee = from.Balance
		}
		from.Balance -= gasFee
		bnfc.Balance += gasFee

		// Ensure these changes get applied.
		db.accounts[fromID] = from
		db.accounts[block.Header.BeneficiaryID] = bnfc

		// Perform basic accounting checks
		{
			if tx.ChainID != db.genesis.ChainID {
				return fmt.Errorf("transaction invalid, wrong chain id, got %d, exp %d", tx.ChainID, db.genesis.ChainID)
			}

			if fromID == tx.ToID {
				return fmt.Errorf("transaction invalid, sending money to yourself, from %s, to %s", fromID, tx.ToID)
			}

			if tx.Nonce <= from.Nonce {
				return fmt.Errorf("transaction invalid, nonce too small, current %d, provided %d", from.Nonce, tx.Nonce)
			}

			if from.Balance == 0 || from.Balance < (tx.Value+tx.Tip) {
				return fmt.Errorf("transaction invalid, insufficient funds, bal %d, needed %d", from.Balance, (tx.Value + tx.Tip))
			}
		}

		// Update the balances between the two parties.
		from.Balance -= tx.Value
		to.Balance += tx.Value

		// Give the beneficiaryID the tip.
		from.Balance -= tx.Tip
		bnfc.Balance += tx.Tip

		// Update the nonce for the next transaction check.
		from.Nonce = tx.Nonce

		// Update the final changes to these accounts.
		db.accounts[fromID] = from
		db.accounts[tx.ToID] = to
		db.accounts[block.Header.BeneficiaryID] = bnfc
	}
	return nil
}

// UpdateLatestBlock provides safe access to update the latest block.
func (db *Database) UpdateLatestBlock(block Block) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.latestBlock = block
}

// LatestBlock returns the latest block.
func (db *Database) LatestBlock() Block {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.latestBlock
}

// Write adds a new block to the chain.
func (db *Database) Write(block Block) error {
	return db.storage.Write(NewBlockData(block))
}

// ForEach returns an iterator to walk through all the blocks
// starting with block number 1.
func (db *Database) ForEach() DatabaseIterator {
	return DatabaseIterator{iterator: db.storage.ForEach()}
}

// GetBlock searches the blockchain on disk to locate and return the
// contents of the specified block by number.
func (db *Database) GetBlock(num uint64) (Block, error) {
	blockData, err := db.storage.GetBlock(num)
	if err != nil {
		return Block{}, err
	}

	return ToBlock(blockData)
}

// /////////////////////////////////////////////////////////////////

// DatabaseIterator provides support for iterating over the blocks in the
// blockchain database using the configured storage option.
type DatabaseIterator struct {
	iterator Iterator
}

// Next retrieves the next block from disk.
func (di *DatabaseIterator) Next() (Block, error) {
	blockData, err := di.iterator.Next()
	if err != nil {
		return Block{}, err
	}

	return ToBlock(blockData)
}

// Done returns the end of chain value.
func (di *DatabaseIterator) Done() bool {
	return di.iterator.Done()
}
