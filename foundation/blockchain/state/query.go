package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// QueryLatest represents a query to the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

// QueryAccount returns a copy of the database record for the specified account.
func (s *State) QueryAccount(account database.AccountID) (database.Account, error) {
	return s.db.Query(account)
}

// QueryBlocksByNumber returns the set of blocks based on block numbers.
// This function reads the blockchain from the disk first.
func (s *State) QueryBlocksByNumber(from, to uint64) []database.Block {
	if from == QueryLatest {
		from = s.db.LatestBlock().Header.Number
		to = from
	}

	if to == QueryLatest {
		to = s.db.LatestBlock().Header.Number
	}

	var out []database.Block
	for i := from; i <= to; i++ {
		block, err := s.db.GetBlock(i)
		if err != nil {
			s.evHandler("state: getblock: ERROR: %s", err)
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

		for _, tx := range block.MerkleTree.Values() {
			if accountID == "" || tx.FromID == accountID || tx.ToID == accountID {
				out = append(out, block)
				break
			}
		}
	}

	return out, nil
}
