package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/signature"
)

// ErrChainForked is returned from the validateNextBlock if another
// node's chain is two or more blocks ahead of ours.
var ErrChainForked = errors.New("blockchain forked, start resync")

// BlockHeader represents common information required for each block.
type BlockHeader struct {
	ParentHash   string  `json:"parent_hash"`   // Hash of the previous block in the chain.
	MinerAccount Account `json:"miner_account"` // The account of the miner who mined the block.
	Difficulty   int     `json:"difficulty"`    // Number of 0's needed to solve the hash solution.
	Number       uint64  `json:"number"`        // The block number in the chain.
	TotalTip     uint    `json:"total_tip"`     // Total tip paid by all senders as an incentive.
	TotalGas     uint    `json:"total_gas"`     // Total gas fee to recover the computation costs paid by the sender.
	TimeStamp    uint64  `json:"time_stamp"`    // Time the block was mined.
	Nonce        uint64  `json:"nonce"`         // Value identified to solve the hash solution.
}

// Block struct represents a grup of transactions batched together.
type Block struct {
	Header       BlockHeader `json:"header"`
	Transactions []BlockTx   `json:"transactions"`
}

// NewBlock constructs a new BlockFS for persisting data.
func NewBlock(minerAccount Account, difficulty, txPerBlock int, parentBlock Block, txs []BlockTx) Block {
	parentHash := signature.ZeroHash
	if parentBlock.Header.Number > 0 {
		parentHash = parentBlock.Hash()
	}
	
	var totalTip uint
	var totalGas uint
	for _, tx := range txs {
		totalTip += tx.Tip
		totalGas += tx.Gas
	}
	
	return Block{
		Header: BlockHeader{
			ParentHash:   parentHash,
			MinerAccount: minerAccount,
			Difficulty:   difficulty,
			Number:       parentBlock.Header.Number + 1,
			TotalTip:     totalTip,
			TotalGas:     totalGas,
			TimeStamp:    uint64(time.Now().UTC().Unix()),
		},
		Transactions: txs,
	}
}

// Hash returns the unique hash for the Block.
func (b Block) Hash() string {
	if b.Header.Number == 0 {
		return signature.ZeroHash
	}
	
	return signature.Hash(b)
}

// ValidateBlock takes a block and validates it to be included into the blockchain.
func (b Block) ValidateBlock(parentBlock Block, evHandler func(v string, args ...any)) (string, error) {
	// The node who sent this block has a chain that is two or more blocks ahead.
	// This means there has been a fork and our block in on the wrong side.
	nextNumber := parentBlock.Header.Number + 1
	if b.Header.Number >= (nextNumber + 2) {
		return signature.ZeroHash, ErrChainForked
	}
	
	evHandler("storage: ValidateBlock: validate: blk[%d]: chain is not forked", b.Header.Number)
	
	if b.Header.Difficulty < parentBlock.Header.Difficulty {
		return signature.ZeroHash, fmt.Errorf("block difficulty is less than parent block difficulty, parent %d, block %d", parentBlock.Header.Difficulty, b.Header.Difficulty)
	}
	
	evHandler("storage: ValidateBlock: validate: blk[%d]: block difficulty is the same or greater than parent block difficulty", b.Header.Number)
	
	hash := b.Hash()
	if !isHashSolved(b.Header.Difficulty, hash) {
		return signature.ZeroHash, fmt.Errorf("%s invalid hash", hash)
	}
	
	evHandler("storage: ValidateBlock: validate: blk[%d]: hash has been solved", b.Header.Number)
	
	// The node who sent this block has a chain that is two or more blocks ahead
	// of ours. This means there has been a fork and we are on the wrong side.
	if b.Header.Number >= (nextNumber + 2) {
		return signature.ZeroHash, ErrChainForked
	}
	
	evHandler("state: ValidateBlock: validate: block number")
	
	if b.Header.Number != nextNumber {
		return signature.ZeroHash, fmt.Errorf("this block is not the next number, got %d, exp %d", b.Header.Number, nextNumber)
	}
	
	evHandler("storage: ValidateBlock: validate: blk[%d]: block number is next number", b.Header.Number)
	
	if b.Header.ParentHash != parentBlock.Hash() {
		return signature.ZeroHash, fmt.Errorf("prev block doesn't match our latest, got %s, exp %s", b.Header.ParentHash, parentBlock.Hash())
	}
	
	evHandler("storage: ValidateBlock: validate: blk[%d]: parent hash does match parent block", b.Header.Number)
	
	if parentBlock.Header.TimeStamp > 0 {
		parentTime := time.Unix(int64(parentBlock.Header.TimeStamp), 0)
		blockTime := time.Unix(int64(b.Header.TimeStamp), 0)
		if !blockTime.After(parentTime) {
			return signature.ZeroHash, fmt.Errorf("block timestamp is before parent block, parent %s, block %s", parentTime, blockTime)
		}
		
		evHandler("storage: ValidateBlock: validate: blk[%d]: block's timestamp is greater than parent block's timestamp", b.Header.Number)
		
		// This is a check that Ethereum does but we can't because we don't run all the time.
		
		// dur := blockTime.Sub(parentTime)
		// if dur.Seconds() > time.Duration(15*time.Second).Seconds() {
		// 	return signature.ZeroHash, fmt.Errorf("block is older than 15 minutes, duration %v", dur)
		// }
		
		// evHandler("storage: ValidateBlock: validate: blk[%d]: block is less than 15 minutes apart from parent block", b.Header.Number)
	}
	
	return hash, nil
}

// PerformPOW does the work of mining to find a valid hash for a
// specified block and returns a BlockFS ready to be written to disk.
func (b Block) PerformPOW(ctx context.Context, difficulty int, ev func(v string, args ...any)) (BlockFS, time.Duration, error) {
	ev("worker: PerformPOW: MINING: started")
	defer ev("worker: PerformPOW: MINING: completed")
	
	for _, tx := range b.Transactions {
		ev("worker: PerformPOW: MINING: tx[%s]", tx)
	}
	
	t := time.Now()
	
	// Choose a random starting point for the nonce.
	nBig, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return BlockFS{}, time.Since(t), ctx.Err()
	}
	b.Header.Nonce = nBig.Uint64()
	
	var attempts uint64
	for {
		attempts++
		if attempts%1_000_000 == 0 {
			ev("worker: PerformPOW: MINING: attempts[%d]", attempts)
		}
		
		// Did we timeout trying to solve the problem.
		if ctx.Err() != nil {
			ev("worker: PerformPOW: MINING: CANCELLED")
			return BlockFS{}, time.Since(t), ctx.Err()
		}
		
		// Hash the block and check if we have solved the puzzle.
		hash := b.Hash()
		if !isHashSolved(difficulty, hash) {
			b.Header.Nonce++
			continue
		}
		
		// Did we timeout trying to solve the problem.
		if ctx.Err() != nil {
			ev("worker: PerformPOW: MINING: CANCELLED")
			return BlockFS{}, time.Since(t), ctx.Err()
		}
		
		ev("worker: PerformPOW: MINING: SOLVED: prevBlk[%s]: newBlk[%s]", b.Header.ParentHash, hash)
		ev("worker: PerformPOW: MINING: attempts[%d]", attempts)
		
		// We found a solution to the POW.
		bfs := BlockFS{
			Hash:  hash,
			Block: b,
		}
		
		return bfs, time.Since(t), nil
	}
}

// isHashSolved checks the hash to make sure it complies with
// the POW rules. We need to match a difficulty number of 0's.
func isHashSolved(difficulty int, hash string) bool {
	const match = "00000000000000000"
	
	if len(hash) != 64 {
		return false
	}
	
	return hash[:difficulty] == match[:difficulty]
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// BlockFS represents what is written to the DB file.
type BlockFS struct {
	Hash  string
	Block Block
}
