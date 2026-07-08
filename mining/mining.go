package mining

import (
	"strings"
	"time"
	"toy-blockchain/block"
	"toy-blockchain/utils"
)

// MineBlock performs Proof-of-Work mining.
// It modifies the block's nonce until its hash satisfies the difficulty rule.
// Returns the final nonce found and the time taken.
func MineBlock(b *block.Block, difficulty int) (int, time.Duration) {
	start := time.Now()
	target := strings.Repeat("0", difficulty)

	for {
		// calculate hash for current block state
		hash := utils.CalculateHash(*b)

		// check if hash meets difficulty requirement
		if len(hash) >= difficulty && hash[:difficulty] == target {
			b.Hash = hash
			elapsed := time.Since(start)
			return b.Nonce, elapsed
		}

		// increase nonce and try again
		b.Nonce++
	}
}