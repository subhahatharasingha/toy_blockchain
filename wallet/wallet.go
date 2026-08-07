package wallet

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Wallet contains an Ed25519 key pair and its blockchain address.
type Wallet struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	Address    string
}

// NewWallet generates a new Ed25519 key pair and derives
// a blockchain address from the public key.
func NewWallet() (*Wallet, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	address := AddressFromPublicKey(publicKey)

	return &Wallet{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
	}, nil
}

// AddressFromPublicKey derives a deterministic address
// by hashing the public key using SHA-256.
func AddressFromPublicKey(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)

	return hex.EncodeToString(hash[:])
}

// Sign signs arbitrary data using the wallet's private key.
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	if len(w.PrivateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key")
	}

	signature := ed25519.Sign(w.PrivateKey, data)

	return signature, nil
}

// Verify verifies that a signature was created by the
// private key corresponding to the supplied public key.
func Verify(
	publicKey ed25519.PublicKey,
	data []byte,
	signature []byte,
) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(publicKey, data, signature)
}
