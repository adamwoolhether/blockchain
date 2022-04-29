package public

import "github.com/adamwoolhether/blockchain/foundation/blockchain/storage"

type acct struct {
	Account storage.AccountID `json:"account"`
	Name    string            `json:"name"`
	Balance uint              `json:"balance"`
	Nonce   uint              `json:"nonce"`
}

type acctInfo struct {
	LatestBlock string `json:"latest_block"`
	Uncommitted int    `json:"uncommitted"`
	Accounts    []acct `json:"database"`
}

type tx struct {
	FromAccount storage.AccountID `json:"from"`
	FromName    string            `json:"from_name"`
	To          storage.AccountID `json:"to"`
	ToName      string            `json:"to_name"`
	Nonce       uint              `json:"nonce"`
	Value       uint              `json:"value"`
	Tip         uint              `json:"tip"`
	Data        []byte            `json:"data"`
	TimeStamp   uint64            `json:"timestamp"`
	Gas         uint              `json:"gas"`
	Sig         string            `json:"sig"`
}

type block struct {
	ParentHash   string            `json:"parent_hash"`
	MinerAccount storage.AccountID `json:"miner_account"`
	Difficulty   int               `json:"difficulty"`
	Number       uint64            `json:"number"`
	TimeStamp    uint64            `json:"timestamp"`
	Nonce        uint64            `json:"nonce"`
	Transactions []tx              `json:"txs"`
}
