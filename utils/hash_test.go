package utils_test

import (
	"testing"
	"toy-blockchain/block"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
)

func TestHashDeterminism(t *testing.T) {
	// Define a test block with identical values
	txs := []transaction.Transaction{
		{Sender: "alice", Receiver: "bob", Amount: 10.5},
		{Sender: "bob", Receiver: "charlie", Amount: 2.0},
	}

	b1 := block.Block{
		Index:        3,
		Timestamp:    1719830400,
		Transactions: txs,
		PreviousHash: "abc123xyz",
		Nonce:        999,
		Hash:         "",
	}

	b2 := block.Block{
		Index:        3,
		Timestamp:    1719830400,
		Transactions: []transaction.Transaction{
			{Sender: "alice", Receiver: "bob", Amount: 10.5},
			{Sender: "bob", Receiver: "charlie", Amount: 2.0},
		},
		PreviousHash: "abc123xyz",
		Nonce:        999,
		Hash:         "some_existing_hash_should_be_ignored",
	}

	hash1 := utils.CalculateHash(b1)
	hash2 := utils.CalculateHash(b2)

	if hash1 == "" {
		t.Fatal("Calculated hash is empty")
	}

	if hash1 != hash2 {
		t.Errorf("Hash calculation is not deterministic.\nHash1: %s\nHash2: %s", hash1, hash2)
	}

	// Double check that multiple serializations yield the same result
	for i := 0; i < 5; i++ {
		h := utils.CalculateHash(b1)
		if h != hash1 {
			t.Fatalf("Hash changed on iteration %d: got %s, expected %s", i, h, hash1)
		}
	}
}
