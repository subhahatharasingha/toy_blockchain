package blockchain_test

import (
	"testing"
	"toy-blockchain/block"
	"toy-blockchain/blockchain"
)

func TestDifficultyRemainsSame(t *testing.T) {
	// Start with blocks having normal block times (60 seconds apart)
	// Base difficulty = 4
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1060, Difficulty: 4},
		{Index: 2, Timestamp: 1120, Difficulty: 4},
		{Index: 3, Timestamp: 1180, Difficulty: 4},
		{Index: 4, Timestamp: 1240, Difficulty: 4},
	}

	nextDiff := blockchain.CalculateNextDifficulty(blocks)
	if nextDiff != 4 {
		t.Errorf("Expected difficulty to remain 4, got %d", nextDiff)
	}
}

func TestDifficultyIncrease(t *testing.T) {
	// Very fast block creation (1 second apart)
	// expectedTime = 5 * 60 = 300 seconds
	// actualTime = 1004 - 1000 = 4 seconds
	// 4 < 150 (expectedTime / 2), so difficulty should increase from 4 to 5
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1001, Difficulty: 4},
		{Index: 2, Timestamp: 1002, Difficulty: 4},
		{Index: 3, Timestamp: 1003, Difficulty: 4},
		{Index: 4, Timestamp: 1004, Difficulty: 4},
	}

	nextDiff := blockchain.CalculateNextDifficulty(blocks)
	if nextDiff != 5 {
		t.Errorf("Expected difficulty to increase to 5, got %d", nextDiff)
	}
}

func TestDifficultyDecrease(t *testing.T) {
	// Very slow block creation (300 seconds apart)
	// expectedTime = 300 seconds
	// actualTime = 2200 - 1000 = 1200 seconds
	// 1200 > 600 (expectedTime * 2), so difficulty should decrease from 4 to 3
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1300, Difficulty: 4},
		{Index: 2, Timestamp: 1600, Difficulty: 4},
		{Index: 3, Timestamp: 1900, Difficulty: 4},
		{Index: 4, Timestamp: 2200, Difficulty: 4},
	}

	nextDiff := blockchain.CalculateNextDifficulty(blocks)
	if nextDiff != 3 {
		t.Errorf("Expected difficulty to decrease to 3, got %d", nextDiff)
	}
}

func TestDifficultyLimits(t *testing.T) {
	// Verify difficulty never goes below 1
	// Start with difficulty 1, and make blocks very slow so difficulty tries to decrease.
	blocksLow := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 1},
		{Index: 1, Timestamp: 1300, Difficulty: 1},
		{Index: 2, Timestamp: 1600, Difficulty: 1},
		{Index: 3, Timestamp: 1900, Difficulty: 1},
		{Index: 4, Timestamp: 2200, Difficulty: 1},
	}

	nextDiffLow := blockchain.CalculateNextDifficulty(blocksLow)
	if nextDiffLow != 1 {
		t.Errorf("Expected difficulty to be clamped to 1, got %d", nextDiffLow)
	}

	// Verify difficulty never exceeds 10
	// Start with difficulty 10, and make blocks very fast so difficulty tries to increase.
	blocksHigh := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 10},
		{Index: 1, Timestamp: 1001, Difficulty: 10},
		{Index: 2, Timestamp: 1002, Difficulty: 10},
		{Index: 3, Timestamp: 1003, Difficulty: 10},
		{Index: 4, Timestamp: 1004, Difficulty: 10},
	}

	nextDiffHigh := blockchain.CalculateNextDifficulty(blocksHigh)
	if nextDiffHigh != 10 {
		t.Errorf("Expected difficulty to be clamped to 10, got %d", nextDiffHigh)
	}
}

func TestDifficultyCorrectIntervalCount(t *testing.T) {
	// Test 1 - Correct interval count
	// N = 5 blocks in AdjustmentWindow.
	// N-1 = 4 intervals. expectedTime = 4 * 60 = 240s.
	// expectedTime/2 = 120s. expectedTime*2 = 480s.

	// Case 1.1: actualTime = 120s (timestamps: 1000, 1030, 1060, 1090, 1120).
	// Since actualTime is NOT < 120, difficulty should NOT increase.
	blocksSameFast := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1030, Difficulty: 4},
		{Index: 2, Timestamp: 1060, Difficulty: 4},
		{Index: 3, Timestamp: 1090, Difficulty: 4},
		{Index: 4, Timestamp: 1120, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksSameFast); diff != 4 {
		t.Errorf("expectedTime boundary failed (actual = 120s): expected difficulty to remain 4, got %d", diff)
	}

	// Case 1.2: actualTime = 119s (timestamps: 1000, 1030, 1060, 1090, 1119).
	// Since actualTime < 120, difficulty MUST increase.
	blocksInc := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1030, Difficulty: 4},
		{Index: 2, Timestamp: 1060, Difficulty: 4},
		{Index: 3, Timestamp: 1090, Difficulty: 4},
		{Index: 4, Timestamp: 1119, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksInc); diff != 5 {
		t.Errorf("expectedTime boundary failed (actual = 119s): expected difficulty to increase to 5, got %d", diff)
	}

	// Case 1.3: actualTime = 480s (timestamps: 1000, 1120, 1240, 1360, 1480).
	// Since actualTime is NOT > 480, difficulty should NOT decrease.
	blocksSameSlow := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1120, Difficulty: 4},
		{Index: 2, Timestamp: 1240, Difficulty: 4},
		{Index: 3, Timestamp: 1360, Difficulty: 4},
		{Index: 4, Timestamp: 1480, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksSameSlow); diff != 4 {
		t.Errorf("expectedTime boundary failed (actual = 480s): expected difficulty to remain 4, got %d", diff)
	}

	// Case 1.4: actualTime = 481s (timestamps: 1000, 1120, 1240, 1360, 1481).
	// Since actualTime > 480, difficulty MUST decrease.
	blocksDec := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1120, Difficulty: 4},
		{Index: 2, Timestamp: 1240, Difficulty: 4},
		{Index: 3, Timestamp: 1360, Difficulty: 4},
		{Index: 4, Timestamp: 1481, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksDec); diff != 3 {
		t.Errorf("expectedTime boundary failed (actual = 481s): expected difficulty to decrease to 3, got %d", diff)
	}
}

func TestDifficultyMiningTooFast(t *testing.T) {
	// Test 2 - Mining too fast (elapsed time = 50s < 120s)
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1010, Difficulty: 4},
		{Index: 2, Timestamp: 1020, Difficulty: 4},
		{Index: 3, Timestamp: 1030, Difficulty: 4},
		{Index: 4, Timestamp: 1050, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocks); diff != 5 {
		t.Errorf("Expected difficulty to increase to 5, got %d", diff)
	}
}

func TestDifficultyMiningTooSlow(t *testing.T) {
	// Test 3 - Mining too slowly (elapsed time = 600s > 480s)
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1150, Difficulty: 4},
		{Index: 2, Timestamp: 1300, Difficulty: 4},
		{Index: 3, Timestamp: 1450, Difficulty: 4},
		{Index: 4, Timestamp: 1600, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocks); diff != 3 {
		t.Errorf("Expected difficulty to decrease to 3, got %d", diff)
	}
}

func TestDifficultyNormalBlockTime(t *testing.T) {
	// Test 4 - Normal block time (elapsed time = 240s, exactly on target)
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1060, Difficulty: 4},
		{Index: 2, Timestamp: 1120, Difficulty: 4},
		{Index: 3, Timestamp: 1180, Difficulty: 4},
		{Index: 4, Timestamp: 1240, Difficulty: 4},
	}
	if diff := blockchain.CalculateNextDifficulty(blocks); diff != 4 {
		t.Errorf("Expected difficulty to remain 4, got %d", diff)
	}
}

func TestDifficultyDeterminism(t *testing.T) {
	// Test 5 - Determinism
	blocks := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 4},
		{Index: 1, Timestamp: 1010, Difficulty: 4},
		{Index: 2, Timestamp: 1020, Difficulty: 4},
		{Index: 3, Timestamp: 1030, Difficulty: 4},
		{Index: 4, Timestamp: 1040, Difficulty: 4},
	}
	res1 := blockchain.CalculateNextDifficulty(blocks)
	res2 := blockchain.CalculateNextDifficulty(blocks)
	res3 := blockchain.CalculateNextDifficulty(blocks)
	if res1 != res2 || res2 != res3 {
		t.Errorf("CalculateNextDifficulty results were not identical: %d, %d, %d", res1, res2, res3)
	}
}

func TestDifficultyClampingBounds(t *testing.T) {
	// Test 6 - Difficulty bounds clamping
	// Min difficulty clamping
	blocksMin := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 1},
		{Index: 1, Timestamp: 1200, Difficulty: 1},
		{Index: 2, Timestamp: 1400, Difficulty: 1},
		{Index: 3, Timestamp: 1600, Difficulty: 1},
		{Index: 4, Timestamp: 2500, Difficulty: 1},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksMin); diff != 1 {
		t.Errorf("Expected difficulty to clamp at 1, got %d", diff)
	}

	// Max difficulty clamping
	blocksMax := []block.Block{
		{Index: 0, Timestamp: 1000, Difficulty: 10},
		{Index: 1, Timestamp: 1001, Difficulty: 10},
		{Index: 2, Timestamp: 1002, Difficulty: 10},
		{Index: 3, Timestamp: 1003, Difficulty: 10},
		{Index: 4, Timestamp: 1004, Difficulty: 10},
	}
	if diff := blockchain.CalculateNextDifficulty(blocksMax); diff != 10 {
		t.Errorf("Expected difficulty to clamp at 10, got %d", diff)
	}
}
