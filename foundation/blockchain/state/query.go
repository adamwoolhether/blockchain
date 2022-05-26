package state

import (
	"errors"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// QueryLatest represents a query to the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

// QueryAccounts returns a copy of the database record for the specified account.
func (s *State) QueryAccounts(account database.AccountID) (database.Account, error) {
	accounts := s.db.CopyAccounts()

	if info, exists := accounts[account]; exists {
		return info, nil
	}

	return database.Account{}, errors.New("not found")
}

// QueryMempoolLength returns the current length of the mempool.
func (s *State) QueryMempoolLength() int {
	return s.mempool.Count()
}

// QueryBlocksByNumber returns the set of blocks based on block numbers.
// This function reads the blockchain from the disk first.
func (s *State) QueryBlocksByNumber(from, to uint64) []database.Block {
	if from == QueryLatest {
		from = s.db.LatestBlock().Header.Number
		to = from
	}

	var out []database.Block
	for i := from; i <= to; i++ {
		block, err := s.db.GetBlock(i)
		if err != nil {
			return nil
		}
		out = append(out, block)
	}

	return out
}

// QueryBlocksByAccount returns the set of blocks by account. If the account
// is empty, all blocks are returns. This function reads the blockchain
// from disk first.
func (s *State) QueryBlocksByAccount(accountID database.AccountID) ([]database.Block, error) {
	var out []database.Block

	iter := s.db.ForEach()
	for block, err := iter.Next(); !iter.Done(); block, err = iter.Next() {
		if err != nil {
			return nil, err
		}

		for _, tx := range block.Transactions.Values() {
			fromID, err := tx.FromAccount()
			if err != nil {
				continue
			}

			if accountID == "" || fromID == accountID || tx.ToID == accountID {
				out = append(out, block)
				break
			}
		}
	}

	return out, nil
}