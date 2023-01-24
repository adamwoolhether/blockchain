package selector

import (
	"sort"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// tipSelect returns transactions with the best tip while respecting
// the nonce for each account/transaction.
var tipSelect = func(m map[database.AccountID][]database.BlockTx, howMany int) []database.BlockTx {
	// Sort the transaction by nonce.
	for key := range m {
		if len(m[key]) > 1 {
			sort.Sort(byNonce(m[key]))
		}
	}

	// Pick the first transaction in the slice for each account. Each
	// iteration represents a new row of selections. Keep doing this
	// until all the transactions have been selected.
	var rows [][]database.BlockTx
	for {
		var row []database.BlockTx
		for key := range m {
			if len(m[key]) > 0 {
				row = append(row, m[key][0])
				m[key] = m[key][1:]
			}
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}

	// Sort each row by tip unless all the transactions from that row
	// are taken. Then try to select the number of requested tranactions.
	// Keep pulling transactions from each row until the amount is
	// fulfilled or there are no more transactions.
	final := []database.BlockTx{}
	for _, row := range rows {
		need := howMany - len(final)
		if len(row) > need {
			sort.Sort(byTip(row))
			final = append(final, row[:need]...)
			break
		}
		final = append(final, row...)
	}

	return final
}
