package state

import (
	"context"
	"errors"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// ErrNoTransactions is returned when a block is requested
// to be created and there aren't enough transactions.
var ErrNoTransactions = errors.New("not enough transactions in mempool")

// /////////////////////////////////////////////////////////////////

// MineNewBlock attempts to create a new block with a proper hash
// that can become the the next block in the chain.
func (s *State) MineNewBlock(ctx context.Context) (database.Block, error) {
	s.evHandler("state: MineNewBlock: MINING: check mempool count")

	// Are there enough transactions in the pool.
	if s.mempool.Count() == 0 {
		return database.Block{}, ErrNoTransactions
	}

	s.evHandler("state: MineNewBlock: MINING: perform POW")

	// CORE NOTE: Hashing the block header and not the whole block so the blockchain
	// can be cryptographically checked by only needing block headers and not full
	// blocks with the transaction data. This will support the ability to have pruned
	// nodes and light clients in the future.
	// - A pruned node stores all the block headers, but only a small number of full
	//   blocks (maybe the last 1000 blocks). This allows for full cryptographic
	//   validation of blocks and transactions without all the extra storage.
	// - A light client keeps block headers and just enough sufficient information
	//   to follow the latest set of blocks being produced. The do not validate
	//   blocks, but can prove a transaction is in a block.

	tx := s.mempool.PickBest(s.genesis.TransPerBlock)

	// Attempt to create a new BlockFS by solving the POW puzzle. This can be cancelled.
	block, err := database.POW(ctx, database.POWArgs{
		BeneficiaryID: s.beneficiaryID,
		Difficulty:    s.genesis.Difficulty,
		MiningReward:  s.genesis.MiningReward,
		PrevBlock:     s.RetrieveLatestBlock(),
		StateRoot:     s.db.HashState(),
		Tx:            tx,
		EvHandler:     s.evHandler,
	})
	if err != nil {
		return database.Block{}, err
	}

	// Just check one more time we were not cancelled.
	if ctx.Err() != nil {
		return database.Block{}, ctx.Err()
	}

	s.evHandler("state: MineNewBlock: MINING: validate and update database")

	// Validate the block and update the blockchain database
	if err := s.validateUpdateDatabase(block); err != nil {
		return database.Block{}, err
	}

	return block, nil
}

// ProcessProposedBlock takes a block received from  a peer, validates
// it, and if it passes, writes the block the local blockchain
func (s *State) ProcessProposedBlock(block database.Block) error {
	s.evHandler("state: ValidateProposedBlock: started: prevBlk[%s]: newBlk[%s]: numTrans[%d]", block.Header.PrevBlockHash, block.Hash(), len(block.Transactions.Values()))
	defer s.evHandler("state: ValidateProposedBlock: completed: newBlk[%s]", block.Hash())

	// Validate the block and then update the blockchain database.
	if err := s.validateUpdateDatabase(block); err != nil {
		return err
	}

	// If the runMiningOperation function is being executed it needs to stop
	// immediately. The G executing runMiningOperation will not return from the
	// function until done is called. That allows this function to complete
	// its state changes before a new mining operation takes place.
	done := s.Worker.SignalCancelMining()
	defer func() {
		s.evHandler("state: ProcessProposedBlock: signal runMiningOperation to terminate")
		done()
	}()

	return nil
}

// /////////////////////////////////////////////////////////////////

// validateUpdateDatabase takes the block and validates it against the
// consensus rules. If the block passes, then the state of the node is
// updated including adding the block to the disk.
func (s *State) validateUpdateDatabase(block database.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evHandler("state: updateLocalState: validate block")

	if err := block.ValidateBlock(s.db.LatestBlock(), s.db.HashState(), s.evHandler); err != nil {
		return err
	}

	s.evHandler("state: updateLocalState: write to disk")

	// Write the new block to the chain on disk.
	if err := s.db.Write(block); err != nil {
		return err
	}
	s.db.UpdateLatestBlock(block)

	s.evHandler("state: updateLocalState: update accounts and remove from mempool")

	// Process the transactions and update the database.
	for _, tx := range block.Transactions.Values() {
		s.evHandler("state: updateLocalState: tx[%s] update and remove", tx)

		// Apply the balance changes based on this transaction.
		if err := s.db.ApplyTx(block, tx); err != nil {
			s.evHandler("state: updateLocalState: WARNING : %s", err)
			continue
		}

		// Remove this transaction from the mempool.
		s.mempool.Delete(tx)
	}

	s.evHandler("state: updateLocalState: apply mining reward")

	// Apply the mining reward for this block.
	s.db.ApplyMiningReward(block)

	return nil
}
