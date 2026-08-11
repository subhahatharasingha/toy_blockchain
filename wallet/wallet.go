package wallet

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

var (
	FaucetPrivateKey ed25519.PrivateKey
	FaucetPublicKey  ed25519.PublicKey
	SystemPrivateKey ed25519.PrivateKey
	SystemPublicKey  ed25519.PublicKey
)

func findKeyPath(filename string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filename
	}
	dir := cwd
	for {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	dir = cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, filename)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filename
}

func init() {
	faucetPath := findKeyPath("faucet.key")
	faucetPrv, err := os.ReadFile(faucetPath)
	if err == nil && len(faucetPrv) == ed25519.PrivateKeySize {
		FaucetPrivateKey = ed25519.PrivateKey(faucetPrv)
		FaucetPublicKey = FaucetPrivateKey.Public().(ed25519.PublicKey)
	} else {
		pub, prv, err := ed25519.GenerateKey(rand.Reader)
		if err == nil {
			FaucetPrivateKey = prv
			FaucetPublicKey = pub
			_ = os.WriteFile(faucetPath, prv, 0600)
		}
	}

	systemPath := findKeyPath("system.key")
	systemPrv, err := os.ReadFile(systemPath)
	if err == nil && len(systemPrv) == ed25519.PrivateKeySize {
		SystemPrivateKey = ed25519.PrivateKey(systemPrv)
		SystemPublicKey = SystemPrivateKey.Public().(ed25519.PublicKey)
	} else {
		pub, prv, err := ed25519.GenerateKey(rand.Reader)
		if err == nil {
			SystemPrivateKey = prv
			SystemPublicKey = pub
			_ = os.WriteFile(systemPath, prv, 0600)
		}
	}
}

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
