package state

import (
	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// UpsertWalletTransaction accepts a transaction from a wallet for inclusion.
func (s *State) UpsertWalletTransaction(signedTx database.SignedTx) error {

	// CORE NOTE: The wallet should ensure the account has a
	// proper balance and nonce. Fees are taken if the tx is mined
	// into a block, even if it doesn't have enough money to pay
	// or the nonce isn't the expected nonce for the account.

	// Check the signed transaction has the proper signature, that the
	// `from` matches the signature, and the `from` and `to` fields are
	// properly formatted.
	if err := signedTx.Validate(s.genesis.ChainID); err != nil {
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

	// Check the signed transaction has the proper signature, that the
	// `from` matches the signature, and the `from` and `to` fields are
	// properly formatted.
	if err := tx.Validate(s.genesis.ChainID); err != nil {
		return err
	}

	if err := s.mempool.Upsert(tx); err != nil {
		return err
	}

	s.Worker.SignalStartMining()

	return nil
}
