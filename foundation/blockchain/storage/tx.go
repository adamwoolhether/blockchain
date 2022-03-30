package storage

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/signature"
)

// UserTx is the transactional data submitted by a user.
type UserTx struct {
	Nonce uint    `json:"nonce"` // *Unique* id for the transaction supplied by the user.
	To    Account `json:"to"`    // Account receiving the transactional benefit.
	Value uint    `json:"value"` // Monetary value received from the transaction.
	Tip   uint    `json:"tip"`   // Tip offered by the sender as an incentive to mine this transaction,..
	Data  []byte  `json:"data"`  // Extra data related to the transaction.
}

// NewUserTx constructs a new user transaction.
func NewUserTx(nonce uint, to Account, value, tip uint, data []byte) (UserTx, error) {
	userTx := UserTx{
		Nonce: nonce,
		To:    to,
		Value: value,
		Tip:   tip,
		Data:  data,
	}
	
	return userTx, nil
}

// Sign uses the specified private key to sign the user transaction.
func (tx UserTx) Sign(privateKey *ecdsa.PrivateKey) (SignedTx, error) {
	// Validate the account in case the UserTx value was hand constructed.
	if !tx.To.IsAccount() {
		return SignedTx{}, fmt.Errorf("to account is not properly formatted")
	}
	
	// Sign the hash with the private key to produce a signature.
	v, r, s, err := signature.Sign(tx, privateKey)
	if err != nil {
		return SignedTx{}, err
	}
	
	// Construct the signed transaction.
	signedTx := SignedTx{
		UserTx: tx,
		V:      v,
		R:      r,
		S:      s,
	}
	
	return signedTx, nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SignedTx is a signed version of the user transaction
// used internally by the blockchain.
type SignedTx struct {
	UserTx
	V *big.Int `json:"v"` // Recovery identifier, either 29 or 30 with ardanID.
	R *big.Int `json:"r"` // First coordinate of the ECDSA signature.
	S *big.Int `json:"s"` // Second coordinate of the ECDSA signature.
}

// FromAccount extracts the account that signed the transaction.
func (tx SignedTx) FromAccount() (Account, error) {
	address, err := signature.FromAddress(tx.UserTx, tx.V, tx.R, tx.S)
	
	return Account(address), err
}

// SignatureString returns the signature as a string.
func (tx SignedTx) SignatureString() string {
	return signature.SignatureString(tx.V, tx.R, tx.S)
}

// String implements the fmt.Stringer interface for logging.
func (tx SignedTx) String() string {
	from, err := tx.FromAccount()
	if err != nil {
		from = "unknown"
	}
	
	return fmt.Sprintf("%s:%d", from, tx.Nonce)
}
