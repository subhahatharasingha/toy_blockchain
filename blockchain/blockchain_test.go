package blockchain_test

import (
	"strings"
	"testing"
	"time"

	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/transaction"
)

// setupTestBlockchain creates an honest chain of 3 blocks (genesis, block 1, block 2)
func setupTestBlockchain(t *testing.T, difficulty int) *blockchain.Blockchain {
	bc := blockchain.NewBlockchain()

	// Add faucet transaction and mine Block 1
	err := bc.AddTransaction(transaction.Transaction{
		Sender:   "faucet",
		Receiver: "alice",
		Amount:   100.0,
	})
	if err != nil {
		t.Fatalf("Failed to add transaction from faucet: %v", err)
	}

	block1, err := bc.CreatePendingBlock(10)
	if err != nil {
		t.Fatalf("CreatePendingBlock failed: %v", err)
	}
	block1.Timestamp = 1719830500 // July 1, 2024 (slightly after genesis)
	mining.MineBlock(&block1, difficulty)
	bc.AddMinedBlock(block1)

	// Add transaction alice -> bob and mine Block 2
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   40.0,
	})
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}

	block2, err := bc.CreatePendingBlock(10)
	if err != nil {
		t.Fatalf("CreatePendingBlock failed: %v", err)
	}
	block2.Timestamp = block1.Timestamp + 10 // sequential timestamp
	mining.MineBlock(&block2, difficulty)
	bc.AddMinedBlock(block2)

	return bc
}

func TestBlockchainValidation(t *testing.T) {
	difficulty := 2
	bc := setupTestBlockchain(t, difficulty)

	// Initial validation on honest chain
	valid, index, err := bc.Validate(difficulty)
	if !valid {
		t.Fatalf("Honest chain validation failed: %v (block index %d)", err, index)
	}

	// Verify balances
	if bal := bc.GetBalance("alice"); bal != 60.0 {
		t.Errorf("Expected alice's balance to be 60.0, got %f", bal)
	}
	if bal := bc.GetBalance("bob"); bal != 40.0 {
		t.Errorf("Expected bob's balance to be 40.0, got %f", bal)
	}
}

func TestBlockchainTamperDetection(t *testing.T) {
	difficulty := 2

	t.Run("Tamper transaction amount in Block 1", func(t *testing.T) {
		bc := setupTestBlockchain(t, difficulty)

		// Modify a transaction inside Block 1
		bc.Blocks[1].Transactions[0].Amount = 150.0 // Modified from 100.0

		valid, offenderIndex, err := bc.Validate(difficulty)
		if valid {
			t.Error("Validation succeeded despite transaction amount tampering!")
		}
		if offenderIndex != 1 {
			t.Errorf("Expected tampering to be detected at block index 1, got %d", offenderIndex)
		}
		if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
			t.Errorf("Expected hash mismatch error, got: %v", err)
		}
	})

	t.Run("Tamper block previous hash link", func(t *testing.T) {
		bc := setupTestBlockchain(t, difficulty)

		// Break the previous hash link of Block 2
		bc.Blocks[2].PreviousHash = "tampered_hash_link"

		valid, offenderIndex, err := bc.Validate(difficulty)
		if valid {
			t.Error("Validation succeeded despite broken previous hash link!")
		}
		if offenderIndex != 2 {
			t.Errorf("Expected tampering to be detected at block index 2, got %d", offenderIndex)
		}
		if err == nil || !strings.Contains(err.Error(), "previous hash mismatch") {
			t.Errorf("Expected previous hash mismatch error, got: %v", err)
		}
	})

	t.Run("Tamper block timestamp out of order", func(t *testing.T) {
		bc := setupTestBlockchain(t, difficulty)

		// Set Block 2's timestamp earlier than Block 1's timestamp
		bc.Blocks[2].Timestamp = bc.Blocks[1].Timestamp - 10

		// Rehash block 2 with the new timestamp so that the hash verification passes,
		// but the chronological timestamp consistency check fails!
		mining.MineBlock(&bc.Blocks[2], difficulty)

		valid, offenderIndex, err := bc.Validate(difficulty)
		if valid {
			t.Error("Validation succeeded despite chronologically invalid timestamp!")
		}
		if offenderIndex != 2 {
			t.Errorf("Expected chronological failure at block index 2, got %d", offenderIndex)
		}
		if err == nil || !strings.Contains(err.Error(), "timestamp") {
			t.Errorf("Expected timestamp validation error, got: %v", err)
		}
	})
}

func TestTransactionRejection(t *testing.T) {
	bc := blockchain.NewBlockchain()
	difficulty := 1

	// Faucet seeds alice with 50 coins
	err := bc.AddTransaction(transaction.Transaction{
		Sender:   "faucet",
		Receiver: "alice",
		Amount:   50.0,
	})
	if err != nil {
		t.Fatalf("Faucet seeding failed: %v", err)
	}

	block1, _ := bc.CreatePendingBlock(10)
	block1.Timestamp = time.Now().Unix()
	mining.MineBlock(&block1, difficulty)
	bc.AddMinedBlock(block1)

	// 1. Try to send negative amount
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   -5.0,
	})
	if err == nil {
		t.Error("Expected rejection for negative transaction amount, but transaction was accepted")
	}

	// 2. Try to send zero amount
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   0.0,
	})
	if err == nil {
		t.Error("Expected rejection for zero transaction amount, but transaction was accepted")
	}

	// 3. Try to overspend immediately (60 coins when balance is 50)
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   60.0,
	})
	if err == nil {
		t.Error("Expected rejection for overspending, but transaction was accepted")
	}

	// 4. Try to spend valid amount (30 coins)
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   30.0,
	})
	if err != nil {
		t.Fatalf("Valid transaction failed to add: %v", err)
	}

	// 5. Try to spend another 30 coins while the first 30 is still pending (should fail due to pending spent tracking)
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   30.0,
	})
	if err == nil {
		t.Error("Expected rejection for cumulative overspending in pending pool, but transaction was accepted")
	}
}
