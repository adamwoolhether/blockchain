package storage

// BlockHeader represents common information required for each block.
type BlockHeader struct {
	ParentHash   string `json:"parent_hash"`   // Hash of the previous block in the chain.
	MinerAccount string `json:"miner_account"` // The account of the miner who mined the block.
	Difficulty   int    `json:"difficulty"`    // Number of 0's needed to solve the hash solution.
	Number       uint64 `json:"number"`        // The block number in the chain.
	TotalTip     uint   `json:"total_tip"`     // Total tip paid by all senders as an incentive.
	TotalGas     uint   `json:"total_gas"`     // Total gas fee to recover the computation costs paid by the sender.
	TimeStamp    uint64 `json:"time_stamp"`    // Time the block was mined.
	None         uint64 `json:"none"`          // Value identified to solve the hash solution.
}

// Block struct represents a grup of transactions batched together.
type Block struct {
	Header       BlockHeader `json:"header"`
	Transactions []UserTx    `json:"transactions"`
}
