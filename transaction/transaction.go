package transaction

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"toy-blockchain/wallet"
)

// Transaction represents a transfer of coins between two blockchain addresses.
type Transaction struct {
	ID        string  `json:"id"`
	Sender    string  `json:"sender"`
	Receiver  string  `json:"receiver"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`

	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// signingPayload contains exactly the fields that are signed.
// ID and Signature are intentionally excluded.
type signingPayload struct {
	Sender    string  `json:"sender"`
	Receiver  string  `json:"receiver"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
	PublicKey string  `json:"public_key"`
}

// idPayload contains the complete signed transaction data used
// to calculate the transaction ID.
type idPayload struct {
	Sender    string  `json:"sender"`
	Receiver  string  `json:"receiver"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
	PublicKey string  `json:"public_key"`
	Signature string  `json:"signature"`
}

// NewSignedTransaction creates and signs a transaction using an Ed25519 wallet.
func NewSignedTransaction(
	senderWallet *wallet.Wallet,
	receiver string,
	amount float64,
) (Transaction, error) {
	if senderWallet == nil {
		return Transaction{}, errors.New("sender wallet cannot be nil")
	}

	publicKey := base64.StdEncoding.EncodeToString(senderWallet.PublicKey)

	tx := Transaction{
		Sender:    senderWallet.Address,
		Receiver:  receiver,
		Amount:    amount,
		Timestamp: time.Now().UnixNano(),
		PublicKey: publicKey,
	}

	signingBytes, err := tx.SigningBytes()
	if err != nil {
		return Transaction{}, err
	}

	signature, err := senderWallet.Sign(signingBytes)
	if err != nil {
		return Transaction{}, err
	}

	tx.Signature = base64.StdEncoding.EncodeToString(signature)

	id, err := tx.CalculateID()
	if err != nil {
		return Transaction{}, err
	}

	tx.ID = id

	return tx, nil
}

// SigningBytes returns a deterministic JSON representation of the
// transaction fields protected by the digital signature.
func (tx Transaction) SigningBytes() ([]byte, error) {
	payload := signingPayload{
		Sender:    tx.Sender,
		Receiver:  tx.Receiver,
		Amount:    tx.Amount,
		Timestamp: tx.Timestamp,
		PublicKey: tx.PublicKey,
	}

	return json.Marshal(payload)
}

// CalculateID creates a SHA-256 identifier from the complete signed
// transaction data, excluding the ID itself.
func (tx Transaction) CalculateID() (string, error) {
	payload := idPayload{
		Sender:    tx.Sender,
		Receiver:  tx.Receiver,
		Amount:    tx.Amount,
		Timestamp: tx.Timestamp,
		PublicKey: tx.PublicKey,
		Signature: tx.Signature,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:]), nil
}

// VerifySignature verifies:
// 1. Public key encoding.
// 2. Signature encoding.
// 3. Sender address belongs to the public key.
// 4. Signature matches the transaction data.
// 5. Transaction ID has not been modified.
func (tx Transaction) VerifySignature() bool {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(tx.PublicKey)
	if err != nil {
		return false
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return false
	}

	publicKey := ed25519.PublicKey(publicKeyBytes)

	expectedAddress := wallet.AddressFromPublicKey(publicKey)
	if expectedAddress != tx.Sender {
		return false
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(tx.Signature)
	if err != nil {
		return false
	}

	if len(signatureBytes) != ed25519.SignatureSize {
		return false
	}

	signingBytes, err := tx.SigningBytes()
	if err != nil {
		return false
	}

	if !wallet.Verify(publicKey, signingBytes, signatureBytes) {
		return false
	}

	expectedID, err := tx.CalculateID()
	if err != nil {
		return false
	}

	return tx.ID == expectedID
}
