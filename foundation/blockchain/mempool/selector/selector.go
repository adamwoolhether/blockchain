// Package selector provides different transaction selecting algorithms.
package selector

import (
	"fmt"
	"strings"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

// List of select strategies.
const (
	StrategyTip         = "tip"
	StrategyTipAdvanced = "tip_advanced"
)

// map of select strategies with functions.
var strategies = map[string]Func{
	StrategyTip:         tipSelect,
	StrategyTipAdvanced: advancedTipSelect,
}

// Func defines a function that takes a mempool of transactions grouped by
// account and selects howMany of them in an order based on the function
// strategy. All selector functions muust respect nonce ordering. Receiving
// -1 for howMany must return all the transactions in the strategy ordering.
type Func func(transactions map[storage.AccountID][]storage.BlockTx, howMany int) []storage.BlockTx

// Retrieve returns the selected strategy function.
func Retrieve(strategy string) (Func, error) {
	fn, exists := strategies[strings.ToLower(strategy)]
	if !exists {
		return nil, fmt.Errorf("strategy %q does not exist", strategy)
	}
	
	return fn, nil
}

// /////////////////////////////////////////////////////////////////
// byNonce provides support to sort transaction by id value. It's methods
// fulfill requirements for sort.Interface.
type byNonce []storage.BlockTx

// Len returns the number of transactions in the list.
func (bn byNonce) Len() int {
	return len(bn)
}

// Less helps sort the list by nonce in ascending order
// to keep transactions in the right order of processing.
func (bn byNonce) Less(i, j int) bool {
	return bn[i].Nonce < bn[j].Nonce
}

// Swap moves the transactions in the order of the nonce value.
func (bn byNonce) Swap(i, j int) {
	bn[i], bn[j] = bn[j], bn[i]
}

// /////////////////////////////////////////////////////////////////
// byTip provides support to sort transactions by tip value,
// it's methods implement sort.Interface.
type byTip []storage.BlockTx

// Len returns the number of transactions in the list.
func (bt byTip) Len() int {
	return len(bt)
}

// Less helps sort the list by tip in ascending order
// to keep transactions in the right order of processing.
func (bt byTip) Less(i, j int) bool {
	return bt[i].Tip < bt[j].Tip
}

// Swap moves the transactions in the order of the tip value.
func (bt byTip) Swap(i, j int) {
	bt[i], bt[j] = bt[j], bt[i]
}
