package mining

import (
	"strings"
	"time"
	"toy-blockchain/block"
	"toy-blockchain/utils"
)


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