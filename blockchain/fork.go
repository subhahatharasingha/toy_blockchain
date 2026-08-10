package blockchain

import (
	"fmt"
	"strings"
	"toy-blockchain/block"
	"toy-blockchain/transaction"
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
// is preferred over the active chain, it replaces the active chain with that fork.
// Returns true if a replacement happened, false otherwise.
func (bc *Blockchain) ResolveForks() bool {
	longestForkIndex := -1

	for i, fork := range bc.Forks {
		if IsPreferredChain(fork, bc.Blocks) {
			longestForkIndex = i
		}
	}

	if longestForkIndex != -1 {
		err := bc.Reorganize(bc.Forks[longestForkIndex])
		if err == nil {
			bc.Forks = append(bc.Forks[:longestForkIndex], bc.Forks[longestForkIndex+1:]...)
			return true
		}
	}

	return false
}

// CalculateCumulativeDifficulty computes the accumulated proof of work (sum of target zeros).
func CalculateCumulativeDifficulty(chain []block.Block) int {
	sum := 0
	for _, b := range chain {
		sum += b.Difficulty
	}
	return sum
}

// IsPreferredChain compares a candidate chain with the current chain.
// Selection rule:
// 1. Valid candidate chain wins first.
// 2. Higher cumulative difficulty (accumulated PoW zeros) wins.
// 3. If cumulative difficulty is equal, the longer chain wins (greater height).
// 4. If both are equal, do not replace the local chain.
func IsPreferredChain(candidate []block.Block, current []block.Block) bool {
	if !ValidateChain(candidate) {
		return false
	}

	candWork := CalculateCumulativeDifficulty(candidate)
	currWork := CalculateCumulativeDifficulty(current)

	if candWork > currWork {
		return true
	}
	if candWork < currWork {
		return false
	}

	// Work is equal, prefer the longer chain (greater height)
	if len(candidate) > len(current) {
		return true
	}

	return false
}

// FindCommonAncestor finds the index of the last block shared by both chains.
func FindCommonAncestor(chainA []block.Block, chainB []block.Block) (int, error) {
	minLen := len(chainA)
	if len(chainB) < minLen {
		minLen = len(chainB)
	}

	ancestorIdx := -1
	for i := 0; i < minLen; i++ {
		if chainA[i].Hash == chainB[i].Hash {
			ancestorIdx = i
		} else {
			break
		}
	}

	if ancestorIdx == -1 {
		return -1, fmt.Errorf("no common ancestor found")
	}

	return ancestorIdx, nil
}

// Reorganize replaces the current blockchain's blocks after a common ancestor with a new preferred chain,
// restoring valid orphaned transactions to the pending pool in a deterministic order.
func (bc *Blockchain) Reorganize(newChain []block.Block) error {
	ancestorIdx, err := FindCommonAncestor(bc.Blocks, newChain)
	if err != nil {
		return err
	}

	// 1. Gather all orphaned transactions from local blocks after the common ancestor
	var orphanedTxs []transaction.Transaction
	for i := ancestorIdx + 1; i < len(bc.Blocks); i++ {
		orphanedTxs = append(orphanedTxs, bc.Blocks[i].Transactions...)
	}

	// 2. Temporarily hold current pending transactions
	oldPending := bc.PendingTransactions

	// 3. Adopt the new chain blocks
	bc.Blocks = newChain

	// 4. Create a map of transaction IDs that are already included in the new adopted chain
	newChainTxIDs := make(map[string]struct{})
	for _, b := range bc.Blocks {
		for _, tx := range b.Transactions {
			if tx.ID != "" {
				newChainTxIDs[tx.ID] = struct{}{}
			}
		}
	}

	// 5. Clear the pending transactions pool and prepare to rebuild it
	bc.PendingTransactions = nil

	// Helper to attempt restoring a transaction
	restoreTx := func(tx transaction.Transaction) {
		if _, inChain := newChainTxIDs[tx.ID]; inChain {
			return
		}
		_ = bc.AddTransaction(tx)
	}

	// Restore orphaned transactions: blocks in chronological order, transactions in block order
	for _, tx := range orphanedTxs {
		restoreTx(tx)
	}

	// Restore previous pending transactions in original order
	for _, tx := range oldPending {
		restoreTx(tx)
	}

	return nil
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
			fmt.Printf("DEBUG: ValidateChain block %d expected difficulty %d, got %d\n", i, expectedDifficulty, current.Difficulty)
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

	// 6. Verify transaction signatures and IDs across all blocks
	for i := 0; i < len(chain); i++ {
		current := chain[i]
		for _, tx := range current.Transactions {
			if tx.ID != "" && !tx.VerifyID() {
				return false
			}
			if tx.Sender == "faucet" || tx.Sender == "system" {
				continue
			}
			if tx.PublicKey == "" || tx.Signature == "" {
				return false
			}
			if !tx.VerifySignature() {
				return false
			}
		}
	}

	return true
}
