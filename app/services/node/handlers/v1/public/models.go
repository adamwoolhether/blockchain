package public

import "github.com/adamwoolhether/blockchain/foundation/blockchain/storage"

type info struct {
	Account storage.Account `json:"account"`
	Name    string          `json:"name"`
	Balance uint            `json:"balance"`
}

type acctInfo struct {
	Uncommitted int    `json:"uncommitted"`
	Accounts    []info `json:"accounts"`
}
