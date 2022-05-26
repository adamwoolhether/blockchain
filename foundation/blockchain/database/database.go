// Package database maintains account balances and other account information.
package database

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/genesis"
)

// Database manages data related to database who have transacted on the blockchain.
type Database struct {
	mu sync.RWMutex

	genesis     genesis.Genesis
	latestBlock Block
	accounts    map[AccountID]Account

	dbPath string
	dbFile *os.File
}

// New Constructs a new account, applies genesis and block information
// and reads/writes the blockchain database on disk if a dbPath is provided.
func New(dbPath string, genesis genesis.Genesis, evHandler func(v string, args ...any)) (*Database, error) {
	var dbFile *os.File

	// If not path is provided, the database will sync only to the
	// genesis information.
	if dbPath != "" {
		var err error
		dbFile, err = os.OpenFile(dbPath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
	}

	db := Database{
		genesis:  genesis,
		accounts: make(map[AccountID]Account),
		dbPath:   dbPath,
		dbFile:   dbFile,
	}

	// Read all the blocks from disk if a path is provided.
	var blocks []Block
	if dbFile != nil {
		var err error
		blocks, err = db.ReadAllBlocks(evHandler, true)
		if err != nil {
			return nil, err
		}
	}

	// Update the database with account balance information from genesis.
	for accountStr, balance := range genesis.Balances {
		accountID, err := ToAccountID(accountStr)
		if err != nil {
			return nil, err
		}
		db.accounts[accountID] = Account{Balance: balance}
	}

	// Set the current latest block in the chain.
	if len(blocks) > 0 {
		db.latestBlock = blocks[len(blocks)-1]
	}

	// Update the databse with account balance information from blocks.
	for _, block := range blocks {
		for _, tx := range block.Transactions.Values() {
			db.ApplyTx(block.Header.Beneficiary, tx)
		}
		db.ApplyMiningReward(block.Header.Beneficiary)
	}

	return &db, nil
}

// Close closes the open blocks database.
func (db *Database) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.dbFile.Close()
}

// Reset re-initializes the database back to the genesis state.
func (db *Database) Reset() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Close and remove the current file.
	db.dbFile.Close()
	if err := os.Remove(db.dbPath); err != nil {
		return err
	}

	// Open a new blockchain database file with create.
	dbFile, err := os.OpenFile(db.dbPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	db.dbFile = dbFile

	// Initialize the database block to the genesis information.
	db.latestBlock = Block{}
	db.accounts = make(map[AccountID]Account)
	for accountStr, balance := range db.genesis.Balances {
		accountID, err := ToAccountID(accountStr)
		if err != nil {
			return err
		}
		db.accounts[accountID] = Account{Balance: balance}
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

// ValidateNonce validates the nonce for the specified transaction is larger
// than the last nonce used by the account who signed the transaction.
func (db *Database) ValidateNonce(tx SignedTx) error {
	from, err := tx.FromAccount()
	if err != nil {
		return err
	}

	var account Account
	db.mu.RLock()
	{
		account = db.accounts[from]
	}
	db.mu.RUnlock()

	if tx.Nonce <= account.Nonce {
		return fmt.Errorf("invalid nonce, got %d, exp > %d", tx.Nonce, account.Nonce)
	}

	return nil
}

// ApplyMiningReward gives the specified miner account the mining reward.
func (db *Database) ApplyMiningReward(beneficiary AccountID) {
	db.mu.Lock()
	defer db.mu.Unlock()

	account := db.accounts[beneficiary]
	account.Balance += db.genesis.MiningReward

	db.accounts[beneficiary] = account
}

// ApplyTx performs the business logic for applying
// a transaction to the database information.
func (db *Database) ApplyTx(minerID AccountID, tx BlockTx) error {

	// Capture the from address from the signature of the transaction.
	fromID, err := tx.FromAccount()
	if err != nil {
		return fmt.Errorf("invalid signature, %s", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	{
		from := db.accounts[fromID]
		to := db.accounts[tx.ToID]
		miner := db.accounts[minerID]

		if fromID == tx.ToID {
			return fmt.Errorf("invalid transaction, sending money to yourself, fromID %s to %s", fromID, tx.ToID)
		}

		if tx.Nonce < from.Nonce {
			return fmt.Errorf("invalid transaction, nonce too small, last %d, tx %d", from.Nonce, tx.Nonce)
		}

		fee := (tx.GasPrice * ts.GasUnits) + tx.Tip
		if tx.Value+fee > from.Balance {
			return fmt.Errorf("%s has an insufficient balance", fromID)
		}

		from.Balance -= tx.Value
		to.Balance += tx.Value

		from.Balance -= fee
		miner.Balance += fee

		from.Nonce = tx.Nonce

		db.accounts[fromID] = from
		db.accounts[tx.ToID] = to
		db.accounts[minerID] = miner
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
func (db *Database) Write(block BlockFS) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Marshal the block for writing to disk.
	blockFSJson, err := json.Marshal(block)
	if err != nil {
		return err
	}

	// Write the new block to the chain on disk.
	if _, err := db.dbFile.Write(append(blockFSJson, '\n')); err != nil {
		return err
	}

	return nil
}

// ReadAllBlocks loads all existing blocks from starts into memory.
// In a real world situation this would require a lot of memory.
func (db *Database) ReadAllBlocks(evHandler func(v string, args ...any), validate bool) ([]Block, error) {
	dbFile, err := os.Open(db.dbPath)
	if err != nil {
		return nil, err
	}
	defer dbFile.Close()

	var blocks []Block
	var latestBlock Block
	scanner := bufio.NewScanner(dbFile)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		var blockFS BlockFS
		if err := json.Unmarshal(scanner.Bytes(), &blockFS); err != nil {
			return nil, err
		}

		block, err := ToBlock(blockFS)
		if err != nil {
			return nil, err
		}

		// We want to skip the block validation for query and retrieve operations.
		if validate {
			if err := block.ValidateBlock(latestBlock, evHandler); err != nil {
				return nil, err
			}
		}

		blocks = append(blocks, block)
		latestBlock = block
	}

	return blocks, nil
}
