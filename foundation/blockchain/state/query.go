package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/accounts"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// QueryLatest represents a query to the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

// QueryAccounts returns a copy of the account informatin by account.
func (s *State) QueryAccounts(account storage.Account) map[storage.Account]accounts.Info {
	cpy := s.accounts.Copy()
	
	final := make(map[storage.Account]accounts.Info)
	if info, exists := cpy[account]; exists {
		final[account] = info
	}
	
	return final
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
		return nil
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
		return nil
	}
	
	var out []storage.Block
blocks:
	for _, block := range blocks {
		for _, tx := range block.Transactions {
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
