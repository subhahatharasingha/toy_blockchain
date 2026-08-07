package transaction_test

import (
	"testing"

	"toy-blockchain/transaction"
	"toy-blockchain/wallet"
)

func TestNewSignedTransaction(t *testing.T) {
	sender, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	receiver, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	tx, err := transaction.NewSignedTransaction(
		sender,
		receiver.Address,
		25.0,
	)
	if err != nil {
		t.Fatalf("failed to create signed transaction: %v", err)
	}

	if tx.ID == "" {
		t.Fatal("expected transaction ID")
	}

	if tx.Sender != sender.Address {
		t.Fatalf(
			"expected sender %s, got %s",
			sender.Address,
			tx.Sender,
		)
	}

	if tx.Receiver != receiver.Address {
		t.Fatalf(
			"expected receiver %s, got %s",
			receiver.Address,
			tx.Receiver,
		)
	}

	if tx.PublicKey == "" {
		t.Fatal("expected public key")
	}

	if tx.Signature == "" {
		t.Fatal("expected signature")
	}

	if !tx.VerifySignature() {
		t.Fatal("expected signed transaction to verify successfully")
	}
}

func TestTransactionSignatureFailsWhenAmountModified(t *testing.T) {
	sender, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	receiver, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	tx, err := transaction.NewSignedTransaction(
		sender,
		receiver.Address,
		10.0,
	)
	if err != nil {
		t.Fatalf("failed to create signed transaction: %v", err)
	}

	tx.Amount = 100.0

	if tx.VerifySignature() {
		t.Fatal("expected modified transaction amount to fail verification")
	}
}

func TestTransactionSignatureFailsWhenReceiverModified(t *testing.T) {
	sender, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	receiver, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	attacker, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create attacker wallet: %v", err)
	}

	tx, err := transaction.NewSignedTransaction(
		sender,
		receiver.Address,
		10.0,
	)
	if err != nil {
		t.Fatalf("failed to create signed transaction: %v", err)
	}

	tx.Receiver = attacker.Address

	if tx.VerifySignature() {
		t.Fatal("expected modified receiver to fail verification")
	}
}

func TestTransactionRejectsMismatchedSenderAddress(t *testing.T) {
	sender, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	receiver, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	attacker, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create attacker wallet: %v", err)
	}

	tx, err := transaction.NewSignedTransaction(
		sender,
		receiver.Address,
		10.0,
	)
	if err != nil {
		t.Fatalf("failed to create signed transaction: %v", err)
	}

	tx.Sender = attacker.Address

	if tx.VerifySignature() {
		t.Fatal("expected mismatched sender address to fail verification")
	}
}

func TestTransactionIDDeterminism(t *testing.T) {
	sender, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create sender wallet: %v", err)
	}

	receiver, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create receiver wallet: %v", err)
	}

	tx, err := transaction.NewSignedTransaction(
		sender,
		receiver.Address,
		15.0,
	)
	if err != nil {
		t.Fatalf("failed to create signed transaction: %v", err)
	}

	id1, err := tx.CalculateID()
	if err != nil {
		t.Fatalf("failed to calculate transaction ID: %v", err)
	}

	id2, err := tx.CalculateID()
	if err != nil {
		t.Fatalf("failed to calculate transaction ID: %v", err)
	}

	if id1 != id2 {
		t.Fatalf(
			"expected deterministic transaction ID, got %s and %s",
			id1,
			id2,
		)
	}

	if id1 != tx.ID {
		t.Fatalf(
			"expected calculated ID %s to match stored ID %s",
			id1,
			tx.ID,
		)
	}
}
