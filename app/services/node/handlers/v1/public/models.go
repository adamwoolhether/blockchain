package public

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

type acct struct {
	Account database.AccountID `json:"account"`
	Name    string             `json:"name"`
	Balance uint               `json:"balance"`
	Nonce   uint               `json:"nonce"`
}

type acctInfo struct {
	LatestBlock string `json:"latest_block"`
	Uncommitted int    `json:"uncommitted"`
	Accounts    []acct `json:"database"`
}

type tx struct {
	FromAccount database.AccountID `json:"from"`
	FromName    string             `json:"from_name"`
	To          database.AccountID `json:"to"`
	ToName      string             `json:"to_name"`
	Nonce       uint               `json:"nonce"`
	Value       uint               `json:"value"`
	Tip         uint               `json:"tip"`
	Data        []byte             `json:"data"`
	TimeStamp   uint64             `json:"timestamp"`
	Gas         uint               `json:"gas"`
	Sig         string             `json:"sig"`
	Proof       []string           `json:"proof"`
	ProofOrder  []int64            `json:"proof_order"`
}

type block struct {
	PrevBlockHash string             `json:"prev_block_hash"`
	Beneficiary   database.AccountID `json:"beneficiary"`
	Difficulty    int                `json:"difficulty"`
	Number        uint64             `json:"number"`
	TimeStamp     uint64             `json:"timestamp"`
	Nonce         uint64             `json:"nonce"`
	TransRoot     string             `json:"trans_roottr"`
	Transactions  []tx               `json:"txs"`
}
