package blockchain_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/transaction"
	"toy-blockchain/wallet"
)

// setupTestBlockchain creates an honest chain of 3 blocks (genesis, block 1, block 2)
func setupTestBlockchain(t *testing.T, difficulty int, aliceWallet, bobWallet *wallet.Wallet) *blockchain.Blockchain {
	bc := blockchain.NewBlockchain()

	// Add faucet transaction and mine Block 1
	err := bc.AddTransaction(transaction.Transaction{
		Sender:   "faucet",
		Receiver: aliceWallet.Address,
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
	mining.MineBlock(&block1, block1.Difficulty)
	bc.AddMinedBlock(block1)

	// Add transaction alice -> bob and mine Block 2
	tx, err := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 40.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	err = bc.AddTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}

	block2, err := bc.CreatePendingBlock(10)
	if err != nil {
		t.Fatalf("CreatePendingBlock failed: %v", err)
	}
	block2.Timestamp = block1.Timestamp + 10 // sequential timestamp
	mining.MineBlock(&block2, block2.Difficulty)
	bc.AddMinedBlock(block2)

	return bc
}

func TestBlockchainValidation(t *testing.T) {
	difficulty := 2
	aliceWallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create alice wallet: %v", err)
	}
	bobWallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create bob wallet: %v", err)
	}

	bc := setupTestBlockchain(t, difficulty, aliceWallet, bobWallet)

	// Initial validation on honest chain
	valid, index, err := bc.Validate(difficulty)
	if !valid {
		t.Fatalf("Honest chain validation failed: %v (block index %d)", err, index)
	}

	// Verify balances
	if bal := bc.GetBalance(aliceWallet.Address); bal != 60.0 {
		t.Errorf("Expected alice's balance to be 60.0, got %f", bal)
	}
	if bal := bc.GetBalance(bobWallet.Address); bal != 40.0 {
		t.Errorf("Expected bob's balance to be 40.0, got %f", bal)
	}
}

func TestBlockchainTamperDetection(t *testing.T) {
	difficulty := 2

	t.Run("Tamper transaction amount in Block 1", func(t *testing.T) {
		aliceWallet, _ := wallet.NewWallet()
		bobWallet, _ := wallet.NewWallet()
		bc := setupTestBlockchain(t, difficulty, aliceWallet, bobWallet)

		// Modify a transaction inside Block 1
		bc.Blocks[1].Transactions[0].Amount = 150.0 // Modified from 100.0

		valid, offenderIndex, err := bc.Validate(difficulty)
		if valid {
			t.Error("Validation succeeded despite transaction amount tampering!")
		}
		if offenderIndex != 1 {
			t.Errorf("Expected tampering to be detected at block index 1, got %d", offenderIndex)
		}
		if err == nil || (!strings.Contains(err.Error(), "hash mismatch") && !strings.Contains(err.Error(), "invalid merkle root")) {
			t.Errorf("Expected hash mismatch or invalid merkle root error, got: %v", err)
		}
	})

	t.Run("Tamper block previous hash link", func(t *testing.T) {
		aliceWallet, _ := wallet.NewWallet()
		bobWallet, _ := wallet.NewWallet()
		bc := setupTestBlockchain(t, difficulty, aliceWallet, bobWallet)

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
		aliceWallet, _ := wallet.NewWallet()
		bobWallet, _ := wallet.NewWallet()
		bc := setupTestBlockchain(t, difficulty, aliceWallet, bobWallet)

		// Set Block 2's timestamp earlier than Block 1's timestamp
		bc.Blocks[2].Timestamp = bc.Blocks[1].Timestamp - 10

		// Rehash block 2 with the new timestamp so that the hash verification passes,
		// but the chronological timestamp consistency check fails!
		mining.MineBlock(&bc.Blocks[2], bc.Blocks[2].Difficulty)

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

func containsTx(pool []transaction.Transaction, tx transaction.Transaction) bool {
	for _, pTx := range pool {
		if pTx.ID == tx.ID {
			return true
		}
	}
	return false
}

func TestTransactionRejection(t *testing.T) {
	bc := blockchain.NewBlockchain()

	aliceWallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create alice wallet: %v", err)
	}
	bobWallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create bob wallet: %v", err)
	}

	// Faucet seeds alice with 50 coins
	err = bc.AddTransaction(transaction.Transaction{
		Sender:   "faucet",
		Receiver: aliceWallet.Address,
		Amount:   50.0,
	})
	if err != nil {
		t.Fatalf("Faucet seeding failed: %v", err)
	}

	block1, _ := bc.CreatePendingBlock(10)
	block1.Timestamp = time.Now().Unix()
	mining.MineBlock(&block1, block1.Difficulty)
	bc.AddMinedBlock(block1)

	// Verify pending pool is empty after mining
	if len(bc.PendingTransactions) != 0 {
		t.Fatalf("Expected empty pending transactions pool, got %d", len(bc.PendingTransactions))
	}

	// Helper to check pool state after a rejection
	checkRejection := func(name string, tx transaction.Transaction, expectedErr string) {
		err := bc.AddTransaction(tx)
		if err == nil {
			t.Errorf("[%s] Expected rejection, but transaction was accepted", name)
		} else if expectedErr != "" && !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("[%s] Expected error containing '%s', got '%v'", name, expectedErr, err)
		}
		if containsTx(bc.PendingTransactions, tx) {
			t.Errorf("[%s] Rejected transaction was found in pending pool", name)
		}
	}

	// 1. Try to send negative amount
	txNeg, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, -5.0)
	checkRejection("Negative Amount", txNeg, "invalid transaction amount")

	// 2. Try to send zero amount
	txZero, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 0.0)
	checkRejection("Zero Amount", txZero, "invalid transaction amount")

	// 3. Try to overspend immediately (60 coins when balance is 50)
	txOver, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 60.0)
	checkRejection("Overspending", txOver, "insufficient balance")

	// 4. Try same sender and receiver
	txSameSenderRecv, _ := transaction.NewSignedTransaction(aliceWallet, aliceWallet.Address, 5.0)
	checkRejection("Same Sender and Receiver", txSameSenderRecv, "sender and receiver cannot be the same account")

	// 5. Try missing public key
	txMissingPubKey, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 5.0)
	txMissingPubKey.PublicKey = ""
	checkRejection("Missing Public Key", txMissingPubKey, "invalid transaction signature")

	// 6. Try missing signature
	txMissingSig, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 5.0)
	txMissingSig.Signature = ""
	checkRejection("Missing Signature", txMissingSig, "invalid transaction signature")

	// 7. Try invalid signature (wrong length/format)
	txInvalidSigStr, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	txInvalidSigStr.Signature = base64.StdEncoding.EncodeToString([]byte("invalid_sig_value_length_64_bytes_padding_mismatch_invalid_verification_value"))
	checkRejection("Invalid Signature (Wrong Length)", txInvalidSigStr, "invalid transaction signature")

	// 8. Try invalid signature (correct length but tampered bytes)
	txInvalidSigTampered, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	sigBytes, _ := base64.StdEncoding.DecodeString(txInvalidSigTampered.Signature)
	if len(sigBytes) > 0 {
		sigBytes[0] ^= 0xFF
	}
	txInvalidSigTampered.Signature = base64.StdEncoding.EncodeToString(sigBytes)
	checkRejection("Invalid Signature (Tampered Bytes)", txInvalidSigTampered, "invalid transaction signature")

	// 9. Try transaction modified after signing (tampered amount)
	txModifiedAmount, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	txModifiedAmount.Amount = 20.0
	checkRejection("Modified Amount After Signing", txModifiedAmount, "invalid transaction signature")

	// 10. Try transaction modified after signing (tampered receiver)
	txModifiedReceiver, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	charlieWallet, _ := wallet.NewWallet()
	txModifiedReceiver.Receiver = charlieWallet.Address
	checkRejection("Modified Receiver After Signing", txModifiedReceiver, "invalid transaction signature")

	// 11. Try to spend valid amount (30 coins)
	txValid1, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 30.0)
	err = bc.AddTransaction(txValid1)
	if err != nil {
		t.Fatalf("Valid transaction failed to add: %v", err)
	}
	if !containsTx(bc.PendingTransactions, txValid1) {
		t.Fatal("Valid transaction was not added to the pending transactions pool")
	}
	if len(bc.PendingTransactions) != 1 {
		t.Fatalf("Expected exactly 1 pending transaction, got %d", len(bc.PendingTransactions))
	}

	// 12. Try to spend another 30 coins while the first 30 is still pending (should fail due to pending spent tracking)
	time.Sleep(10 * time.Millisecond)
	txValid2, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 30.0)
	checkRejection("Cumulative Overspending", txValid2, "insufficient balance")

	// Final check: pending transactions should only contain txValid1
	if len(bc.PendingTransactions) != 1 || bc.PendingTransactions[0].ID != txValid1.ID {
		t.Error("Pending transactions pool was corrupted by rejected transactions")
	}
}
