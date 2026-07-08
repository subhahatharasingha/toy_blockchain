package mining_test

import (
	"strings"
	"testing"
	"toy-blockchain/block"
	"toy-blockchain/mining"
)

func TestMiningDifficulty(t *testing.T) {
	b := block.Block{
		Index:        1,
		Timestamp:    1719830400,
		Transactions: nil,
		PreviousHash: "genesis_hash",
		Nonce:        0,
		Hash:         "",
	}

	difficulty := 3
	target := strings.Repeat("0", difficulty)

	nonce, elapsed := mining.MineBlock(&b, difficulty)

	if elapsed <= 0 {
		t.Errorf("Expected positive elapsed duration, got %v", elapsed)
	}

	if nonce != b.Nonce {
		t.Errorf("Returned nonce %d doesn't match block nonce %d", nonce, b.Nonce)
	}

	if len(b.Hash) < difficulty || b.Hash[:difficulty] != target {
		t.Errorf("Block hash '%s' does not satisfy difficulty target %d (expected prefix '%s')", b.Hash, difficulty, target)
	}
}
