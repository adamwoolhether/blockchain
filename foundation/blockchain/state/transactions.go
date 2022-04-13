package state

import "github.com/adamwoolhether/blockchain/foundation/blockchain/storage"

// SubmitWalletTransaction accepts a transaction from a wallet for inclusion.
func (s *State) SubmitWalletTransaction(signedTx storage.SignedTx) error {
	if err := s.validateTransaction(signedTx); err != nil {
		return err
	}
	
	tx := storage.NewBlockTx(signedTx, s.genesis.GasPrice)
	
	n, err := s.mempool.Upsert(tx)
	if err != nil {
		return err
	}
	
	s.Worker.SignalShareTx(tx)
	
	if n >= s.genesis.TxsPerBlock {
		s.Worker.SignalStartMining()
	}
	
	return nil
}

// SubmitNodeTransaction accepts a transaction from a node for inclusion.
func (s *State) SubmitNodeTransaction(tx storage.BlockTx) error {
	if err := s.validateTransaction(tx.SignedTx); err != nil {
		return err
	}
	
	n, err := s.mempool.Upsert(tx)
	if err != nil {
		return err
	}
	
	if n >= s.genesis.TxsPerBlock {
		s.Worker.SignalStartMining()
	}
	
	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// validateTransaction takes the signed transaction and validates
// it has a proper signature and other aspects of the data.
func (s *State) validateTransaction(signedTx storage.SignedTx) error {
	if err := signedTx.Validate(); err != nil {
		return err
	}
	
	if err := s.accounts.ValidateNonce(signedTx); err != nil {
		return err
	}
	
	return nil
}
