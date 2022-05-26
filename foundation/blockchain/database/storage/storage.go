package storage

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
	"github.com/adamwoolhether/blockchain/foundation/blockchain/merkle"
)

// Block represents what serialized to disk.
type Block struct {
	Hash   string               `json:"hash"`
	Header database.BlockHeader `json:"Block"`
	Trans  []database.BlockTx   `json:"trans"`
}

// NewBlock constructs a Block that can be serialized to disk.
func NewBlock(dbBlock database.Block) Block {
	block := Block{
		Hash:   dbBlock.Hash(),
		Header: dbBlock.Header,
		Trans:  dbBlock.Transactions.Values(),
	}

	return block
}

// ToDatabaseBlock converts a storage Block into a database Block.
func ToDatabaseBlock(block Block) (database.Block, error) {
	tree, err := merkle.NewTree(block.Trans)
	if err != nil {
		return database.Block{}, err
	}

	dbBlock := database.Block{
		Header:       block.Header,
		Transactions: tree,
	}

	return dbBlock, nil
}
