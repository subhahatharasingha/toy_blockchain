package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"toy-blockchain/block"
	"toy-blockchain/transaction"
)

// blockHashInput defines the exact subset of Block fields that feed the hash.
type blockHashInput struct {
	Index        int                       `json:"index"`
	Timestamp    int64                     `json:"timestamp"`
	Transactions []transaction.Transaction `json:"transactions"`
	PreviousHash string                    `json:"previousHash"`
	Nonce        int                       `json:"nonce"`
}

// CalculateHash generates a SHA256 hash for a block by serializing
// its fields (excluding the hash itself) to JSON and hashing the output.
func CalculateHash(b block.Block) string {
	input := blockHashInput{
		Index:        b.Index,
		Timestamp:    b.Timestamp,
		Transactions: b.Transactions,
		PreviousHash: b.PreviousHash,
		Nonce:        b.Nonce,
	}

	// Convert the block input into JSON (struct fields are marshaled deterministically in order)
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}

	// Calculate SHA256
	hash := sha256.Sum256(data)

	// Return hexadecimal string
	return hex.EncodeToString(hash[:])
}