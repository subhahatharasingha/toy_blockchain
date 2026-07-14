package blockchain

import (
	"strings"
	"toy-blockchain/block"
	"toy-blockchain/utils"
)

// AddFork appends a competing chain branch to the alternative branches list.
func (bc *Blockchain) AddFork(chain []block.Block) {
	bc.Forks = append(bc.Forks, chain)
}

// IsLongerChain checks if candidate length is greater than current length.
func IsLongerChain(candidate []block.Block, current []block.Block) bool {
	return len(candidate) > len(current)
}

// ResolveForks iterates through alternative forks. If it finds a valid fork that
// is longer than the active chain, it replaces the active chain with that fork.
// Returns true if a replacement happened, false otherwise.
func (bc *Blockchain) ResolveForks() bool {
	longestForkIndex := -1
	longestForkLength := len(bc.Blocks)

	for i, fork := range bc.Forks {
		if ValidateChain(fork) && IsLongerChain(fork, bc.Blocks) {
			if len(fork) > longestForkLength {
				longestForkLength = len(fork)
				longestForkIndex = i
			}
		}
	}

	if longestForkIndex != -1 {
		// Replace current chain with the longest valid fork
		bc.Blocks = bc.Forks[longestForkIndex]
		// Remove the selected fork from the list
		bc.Forks = append(bc.Forks[:longestForkIndex], bc.Forks[longestForkIndex+1:]...)
		return true
	}

	return false
}

// ValidateChain checks the dynamic validity of a given block slice.
func ValidateChain(chain []block.Block) bool {
	if len(chain) == 0 {
		return false
	}

	// Verify Genesis Block
	genesis := chain[0]
	if genesis.Index != 0 {
		return false
	}
	if genesis.PreviousHash != "0" {
		return false
	}
	calculatedGenesisMerkleRoot := utils.CalculateMerkleRoot(genesis.Transactions)
	if genesis.MerkleRoot != calculatedGenesisMerkleRoot {
		return false
	}
	expectedGenesisHash := utils.CalculateHash(genesis)
	if genesis.Hash != expectedGenesisHash {
		return false
	}

	// Verify Subsequent Blocks
	for i := 1; i < len(chain); i++ {
		current := chain[i]
		previous := chain[i-1]

		// 0. Check difficulty adjustment sequence
		expectedDifficulty := CalculateNextDifficulty(chain[:i])
		if current.Difficulty != expectedDifficulty {
			return false
		}

		// 0. Check merkle root
		calculatedMerkleRoot := utils.CalculateMerkleRoot(current.Transactions)
		if current.MerkleRoot != calculatedMerkleRoot {
			return false
		}

		// 1. Check index sequentiality
		if current.Index != i {
			return false
		}

		// 2. Previous hash link must match
		if current.PreviousHash != previous.Hash {
			return false
		}

		// 3. Stored hash must match recalculated hash
		recalculatedHash := utils.CalculateHash(current)
		if current.Hash != recalculatedHash {
			return false
		}

		// 4. Proof of work must be valid
		blockTarget := strings.Repeat("0", current.Difficulty)
		if len(current.Hash) < current.Difficulty || current.Hash[:current.Difficulty] != blockTarget {
			return false
		}

		// 5. Check timestamp consistency (chronological order)
		if current.Timestamp < previous.Timestamp {
			return false
		}
	}

	// 6. Verify transaction signatures across all blocks
	for i := 0; i < len(chain); i++ {
		current := chain[i]
		for _, tx := range current.Transactions {
			if tx.Sender == "faucet" || tx.Sender == "system" {
				continue
			}
			if tx.PublicKey == "" || tx.Signature == "" {
				continue
			}
			if !utils.VerifyTransactionSignature(tx) {
				return false
			}
		}
	}

	return true
}
