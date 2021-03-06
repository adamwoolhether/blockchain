package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// UpsertWalletTransaction accepts a transaction from a wallet for inclusion.
func (s *State) UpsertWalletTransaction(signedTx database.SignedTx) error {

	// CORE NOTE: Check the signed transaction has proper signature and valid
	// account for the recipient. The wallet should ensure the
	// account has a proper balance and nonce. Fees are taken if
	// the tx is mined into block.
	if err := signedTx.Validate(); err != nil {
		return err
	}

	const oneUnitofGas = 1
	tx := database.NewBlockTx(signedTx, s.genesis.GasPrice, oneUnitofGas)
	if err := s.mempool.Upsert(tx); err != nil {
		return err
	}

	s.Worker.SignalShareTx(tx)
	s.Worker.SignalStartMining()

	return nil
}

// UpsertNodeTransaction accepts a transaction from a node for inclusion.
func (s *State) UpsertNodeTransaction(tx database.BlockTx) error {

	// Check the signed transaction has a proper signature and
	// valid account for the signature.
	if err := tx.Validate(); err != nil {
		return err
	}

	if err := s.mempool.Upsert(tx); err != nil {
		return err
	}

	s.Worker.SignalStartMining()

	return nil
}
