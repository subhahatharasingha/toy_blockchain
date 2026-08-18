package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"toy-blockchain/transaction"
)

// hashTransaction converts a transaction into deterministic string data
// and generates its SHA-256 hash in hex representation.
func hashTransaction(tx transaction.Transaction) string {
	data := fmt.Sprintf("%s:%s:%f:%s:%s", tx.Sender, tx.Receiver, tx.Amount, tx.PublicKey, tx.Signature)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CalculateMerkleRoot calculates the Merkle root for a slice of transactions.
// It returns an empty string if there are no transactions.
// If there is only one transaction, it returns its hash.
// For multiple transactions, it builds the Merkle tree recursively or iteratively
// and handles odd-length levels by duplicating the last hash.
func CalculateMerkleRoot(transactions []transaction.Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	// Initialize the leaf hashes
	var level []string
	for _, tx := range transactions {
		level = append(level, hashTransaction(tx))
	}

	// Iteratively combine hashes until only one remains
	for len(level) > 1 {
		var nextLevel []string
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				// Pair exists
				combined := level[i] + level[i+1]
				hash := sha256.Sum256([]byte(combined))
				nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
			} else {
				// Odd element: duplicate the last hash
				combined := level[i] + level[i]
				hash := sha256.Sum256([]byte(combined))
				nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
			}
		}
		level = nextLevel
	}

	return level[0]
}

// MerkleProofStep represents one step in the Merkle inclusion proof.
type MerkleProofStep struct {
	Hash     string `json:"hash"`
	Position string `json:"position"` // "left" or "right"
}

// GenerateMerkleProof generates the Merkle proof for a transaction at txIndex in a slice of transactions.
func GenerateMerkleProof(transactions []transaction.Transaction, txIndex int) ([]MerkleProofStep, error) {
	if len(transactions) == 0 {
		return nil, fmt.Errorf("no transactions to generate proof for")
	}
	if txIndex < 0 || txIndex >= len(transactions) {
		return nil, fmt.Errorf("transaction index %d out of bounds (0-%d)", txIndex, len(transactions)-1)
	}

	var proof []MerkleProofStep

	// Initialize the leaf hashes
	var level []string
	for _, tx := range transactions {
		level = append(level, hashTransaction(tx))
	}

	currentIndex := txIndex

	for len(level) > 1 {
		var siblingHash string
		var siblingPosition string

		// Determine sibling and next index
		if currentIndex%2 == 0 {
			// Even index: sibling is to the right (currentIndex + 1)
			if currentIndex+1 < len(level) {
				siblingHash = level[currentIndex+1]
			} else {
				// Odd element at the end: sibling is itself (duplicate)
				siblingHash = level[currentIndex]
			}
			siblingPosition = "right"
		} else {
			// Odd index: sibling is to the left (currentIndex - 1)
			siblingHash = level[currentIndex-1]
			siblingPosition = "left"
		}

		proof = append(proof, MerkleProofStep{
			Hash:     siblingHash,
			Position: siblingPosition,
		})

		// Construct next level
		var nextLevel []string
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				combined := level[i] + level[i+1]
				hash := sha256.Sum256([]byte(combined))
				nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
			} else {
				combined := level[i] + level[i]
				hash := sha256.Sum256([]byte(combined))
				nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
			}
		}

		level = nextLevel
		currentIndex = currentIndex / 2
	}

	return proof, nil
}

// VerifyMerkleProof verifies a Merkle proof against a given Merkle root.
func VerifyMerkleProof(tx transaction.Transaction, proof []MerkleProofStep, txIndex int, merkleRoot string) bool {
	hash := hashTransaction(tx)

	for _, step := range proof {
		var combined string
		if step.Position == "right" {
			combined = hash + step.Hash
		} else if step.Position == "left" {
			combined = step.Hash + hash
		} else {
			return false // Invalid position value
		}
		sha := sha256.Sum256([]byte(combined))
		hash = hex.EncodeToString(sha[:])
	}

	return hash == merkleRoot
}
