package blockchain

import (
	"toy-blockchain/block"
)

const AdjustmentWindow = 5
const TargetBlockTime = 60 // in seconds

// CalculateNextDifficulty determines the mining difficulty for the next block.
// It uses the mining speed of the last 5 blocks to decide whether to
// increase, decrease, or keep the difficulty, clamped between 1 and 10.
func CalculateNextDifficulty(blocks []block.Block) int {
	if len(blocks) == 0 {
		return 1
	}

	latestBlock := blocks[len(blocks)-1]

	// If there are less than 5 blocks, keep the latest block's difficulty
	if len(blocks) < AdjustmentWindow {
		difficulty := latestBlock.Difficulty
		if difficulty < 1 {
			return 1
		}
		if difficulty > 10 {
			return 10
		}
		return difficulty
	}

	// Get the last 5 blocks (from len(blocks)-5 to len(blocks)-1)
	oldestBlock := blocks[len(blocks)-AdjustmentWindow]

	actualTime := latestBlock.Timestamp - oldestBlock.Timestamp
	expectedTime := int64((AdjustmentWindow - 1) * TargetBlockTime)

	difficulty := latestBlock.Difficulty

	if actualTime < expectedTime/2 {
		difficulty++
	} else if actualTime > expectedTime*2 {
		difficulty--
	}

	// Clamp difficulty to the range [1, 10]
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 10 {
		difficulty = 10
	}

	return difficulty
}
