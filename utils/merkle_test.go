package utils_test

import (
	"testing"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
)

func TestMerkleRootFromMultipleTransactions(t *testing.T) {
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
		{Sender: "Bob", Receiver: "Charlie", Amount: 5.0, PublicKey: "pub2", Signature: "sig2"},
		{Sender: "Charlie", Receiver: "Alice", Amount: 2.5, PublicKey: "pub3", Signature: "sig3"},
	}

	root := utils.CalculateMerkleRoot(txs)
	if root == "" {
		t.Fatal("Expected Merkle root from multiple transactions to not be empty")
	}
}

func TestMerkleRootChangesOnTransactionTampering(t *testing.T) {
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
		{Sender: "Bob", Receiver: "Charlie", Amount: 5.0, PublicKey: "pub2", Signature: "sig2"},
	}

	rootOriginal := utils.CalculateMerkleRoot(txs)
	if rootOriginal == "" {
		t.Fatal("Expected original Merkle root to not be empty")
	}

	// Change amount on one transaction
	txs[1].Amount = 50.0

	rootTampered := utils.CalculateMerkleRoot(txs)
	if rootTampered == "" {
		t.Fatal("Expected tampered Merkle root to not be empty")
	}

	if rootOriginal == rootTampered {
		t.Errorf("Expected Merkle root to change when transaction amount is modified, but got identical root: %s", rootOriginal)
	}
}

func TestMerkleRootSingleTransaction(t *testing.T) {
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
	}

	root := utils.CalculateMerkleRoot(txs)
	if root == "" {
		t.Fatal("Expected Merkle root for a single transaction to not be empty")
	}
}

func TestMerkleProofsValidAndTampered(t *testing.T) {
	// Setup transactions (Even count: 4)
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
		{Sender: "Bob", Receiver: "Charlie", Amount: 5.0, PublicKey: "pub2", Signature: "sig2"},
		{Sender: "Charlie", Receiver: "David", Amount: 2.5, PublicKey: "pub3", Signature: "sig3"},
		{Sender: "David", Receiver: "Eve", Amount: 1.25, PublicKey: "pub4", Signature: "sig4"},
	}

	root := utils.CalculateMerkleRoot(txs)

	// Test 1: Valid Proofs for all transactions
	for i, tx := range txs {
		proof, err := utils.GenerateMerkleProof(txs, i)
		if err != nil {
			t.Fatalf("Failed to generate Merkle proof for index %d: %v", i, err)
		}
		if !utils.VerifyMerkleProof(tx, proof, i, root) {
			t.Errorf("Expected Merkle proof to verify successfully for index %d", i)
		}
	}

	// Test 2: Modified Transaction
	proof, _ := utils.GenerateMerkleProof(txs, 0)
	tamperedTx := txs[0]
	tamperedTx.Amount = 999.0
	if utils.VerifyMerkleProof(tamperedTx, proof, 0, root) {
		t.Error("Expected verification to fail for modified transaction amount")
	}

	tamperedTx2 := txs[0]
	tamperedTx2.Receiver = "Malicious"
	if utils.VerifyMerkleProof(tamperedTx2, proof, 0, root) {
		t.Error("Expected verification to fail for modified transaction receiver")
	}

	// Test 3: Modified Proof
	tamperedProof := make([]utils.MerkleProofStep, len(proof))
	copy(tamperedProof, proof)
	if len(tamperedProof) > 0 {
		tamperedProof[0].Hash = "0000000000000000000000000000000000000000000000000000000000000000"
		if utils.VerifyMerkleProof(txs[0], tamperedProof, 0, root) {
			t.Error("Expected verification to fail for modified proof hash")
		}
	}

	// Test 4: Wrong Root
	wrongRoot := "1111111111111111111111111111111111111111111111111111111111111111"
	if utils.VerifyMerkleProof(txs[0], proof, 0, wrongRoot) {
		t.Error("Expected verification to fail for wrong Merkle root")
	}

	// Test 5: Wrong Index
	// Note: VerifyMerkleProof takes the index but verifies mathematically by hashing.
	// If we use the proof generated for index 0 to verify the transaction at index 1, it must fail.
	if utils.VerifyMerkleProof(txs[1], proof, 1, root) {
		t.Error("Expected verification to fail when using a proof from a different index")
	}
}

func TestMerkleProofsOddTransactionCount(t *testing.T) {
	// Test 6: Odd count (3 transactions)
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
		{Sender: "Bob", Receiver: "Charlie", Amount: 5.0, PublicKey: "pub2", Signature: "sig2"},
		{Sender: "Charlie", Receiver: "Alice", Amount: 2.5, PublicKey: "pub3", Signature: "sig3"},
	}

	root := utils.CalculateMerkleRoot(txs)

	for i, tx := range txs {
		proof, err := utils.GenerateMerkleProof(txs, i)
		if err != nil {
			t.Fatalf("Failed to generate proof for index %d in odd-length transactions: %v", i, err)
		}
		if !utils.VerifyMerkleProof(tx, proof, i, root) {
			t.Errorf("Expected Merkle proof to verify successfully for index %d in odd-length transactions", i)
		}
	}
}

func TestMerkleProofsEvenTransactionCount(t *testing.T) {
	// Test 7: Even count (4 transactions)
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
		{Sender: "Bob", Receiver: "Charlie", Amount: 5.0, PublicKey: "pub2", Signature: "sig2"},
		{Sender: "Charlie", Receiver: "David", Amount: 2.5, PublicKey: "pub3", Signature: "sig3"},
		{Sender: "David", Receiver: "Alice", Amount: 1.25, PublicKey: "pub4", Signature: "sig4"},
	}

	root := utils.CalculateMerkleRoot(txs)

	for i, tx := range txs {
		proof, err := utils.GenerateMerkleProof(txs, i)
		if err != nil {
			t.Fatalf("Failed to generate proof for index %d in even-length transactions: %v", i, err)
		}
		if !utils.VerifyMerkleProof(tx, proof, i, root) {
			t.Errorf("Expected Merkle proof to verify successfully for index %d in even-length transactions", i)
		}
	}
}

func TestMerkleProofsSingleTransaction(t *testing.T) {
	// Test 8: Single transaction
	txs := []transaction.Transaction{
		{Sender: "Alice", Receiver: "Bob", Amount: 10.0, PublicKey: "pub1", Signature: "sig1"},
	}

	root := utils.CalculateMerkleRoot(txs)

	proof, err := utils.GenerateMerkleProof(txs, 0)
	if err != nil {
		t.Fatalf("Failed to generate proof for single transaction: %v", err)
	}

	if len(proof) != 0 {
		t.Errorf("Expected proof for single transaction to be empty, got length %d", len(proof))
	}

	if !utils.VerifyMerkleProof(txs[0], proof, 0, root) {
		t.Error("Expected Merkle proof to verify successfully for single transaction block")
	}
}
