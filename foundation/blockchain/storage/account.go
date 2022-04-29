package storage

import (
	"crypto/ecdsa"
	"errors"
	
	"github.com/ethereum/go-ethereum/crypto"
)

// Account represents an account in the system that can
// sign and is associated with transactions on the blockchain.
type Account string

// ToAccount converts a hex-encoded string to an account and
// validates the hex-encoded string is formatted correctly.
func ToAccount(hex string) (Account, error) {
	a := Account(hex)
	if !a.IsAccount() {
		return "", errors.New("invalid account format")
	}
	
	return a, nil
}

// PublicKeyToAccount converts the public key to an account value.
func PublicKeyToAccount(pk ecdsa.PublicKey) Account {
	return Account(crypto.PubkeyToAddress(pk).String())
}

// IsAccount verifies whether the underlying data represents
// a valid hex-encoded account.
func (a Account) IsAccount() bool {
	const addressLength = 20
	
	if has0xPrefix(a) {
		a = a[2:]
	}
	
	return len(a) == 2*addressLength && isHex(a)
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// has0xPrefix validates the accounts starts with 0x.
func has0xPrefix(a Account) bool {
	return len(a) >= 2 && a[0] == '0' && (a[1] == 'x' || a[1] == 'X')
}

// isHex validates whether each byte is a valid hexadecimal string
func isHex(a Account) bool {
	if len(a)%2 != 0 {
		return false
	}
	
	for _, c := range []byte(a) {
		if !isHexCharacter(c) {
			return false
		}
	}
	
	return true
}

// isHexCharacter returns a bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}
