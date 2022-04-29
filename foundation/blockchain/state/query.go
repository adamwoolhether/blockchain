package state

import (
	"errors"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// QueryLatest represents a query to the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

// QueryDatabaseRecord returns a copy of the database record for the specified account.
func (s *State) QueryDatabaseRecord(account storage.Account) (database.Info, error) {
	records := s.db.CopyRecords()
	
	if info, exists := records[account]; exists {
		return info, nil
	}
	
	return database.Info{}, errors.New("not found")
}

// QueryMempoolLength returns the current length of the mempool.
func (s *State) QueryMempoolLength() int {
	return s.mempool.Count()
}

// QueryBlocksByNumber returns the set of blocks based on block numbers.
// This function reads the blockchain from the disk first.
func (s *State) QueryBlocksByNumber(from, to uint64) []storage.Block {
	blocks, err := s.storage.ReadAllBlocks(s.evHandler, false)
	if err != nil {
		return []storage.Block{}
	}
	
	if from == QueryLatest {
		from = blocks[len(blocks)-1].Header.Number
		to = from
	}
	
	var out []storage.Block
	for _, block := range blocks {
		if block.Header.Number >= from && block.Header.Number <= to {
			out = append(out, block)
		}
	}
	
	return out
}

// QueryBlocksByAccount returns the set of blocks by account. If the account
// is empty, all blocks are returns. This function reads the blockchain
// from disk first.
func (s *State) QueryBlocksByAccount(account storage.Account) []storage.Block {
	blocks, err := s.storage.ReadAllBlocks(s.evHandler, false)
	if err != nil {
		return []storage.Block{}
	}
	
	var out []storage.Block
blocks:
	for _, block := range blocks {
		for _, tx := range block.Transactions.Values() {
			from, err := tx.FromAccount()
			if err != nil {
				continue
			}
			if account == "" || from == account || tx.To == account {
				out = append(out, block)
				continue blocks
			}
		}
	}
	
	return out
}
