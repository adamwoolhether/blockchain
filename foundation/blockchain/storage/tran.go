package storage

// UserTx is the transactional data submitted by a user.
type UserTx struct {
	Nonce uint   `json:"nonce"` // *Unique* id for the transaction supplied by the user.
	From  string `json:"from"`  // Account sending the money.
	To    string `json:"to"`    // Account receiving the transactional benefit.
	Value uint   `json:"value"` // Monetary value received from the transaction.
	Tip   uint   `json:"tip"`   // Tip offered by the sender as an incentive to mine this transaction,..
	Data  []byte `json:"data"`  // Extra data related to the transaction.
}

// NewUserTx constructs a new user transaction.
func NewUserTx(nonce uint, from, to string, value, tip uint, data []byte) (UserTx, error) {
	userTx := UserTx{
		Nonce: nonce,
		From:  from,
		To:    to,
		Value: value,
		Tip:   tip,
		Data:  data,
	}
	
	return userTx, nil
}
