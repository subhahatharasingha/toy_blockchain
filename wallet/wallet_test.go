package wallet_test

import (
	"testing"

	"toy-blockchain/wallet"
)

func TestNewWallet(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	if len(w.PrivateKey) == 0 {
		t.Fatal("expected private key to be generated")
	}

	if len(w.PublicKey) == 0 {
		t.Fatal("expected public key to be generated")
	}

	if w.Address == "" {
		t.Fatal("expected wallet address to be generated")
	}
}

func TestAddressDeterminism(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	address1 := wallet.AddressFromPublicKey(w.PublicKey)
	address2 := wallet.AddressFromPublicKey(w.PublicKey)

	if address1 != address2 {
		t.Fatalf(
			"expected deterministic address, got %s and %s",
			address1,
			address2,
		)
	}
}

func TestSignAndVerify(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	data := []byte("alice sends 10 coins to bob")

	signature, err := w.Sign(data)
	if err != nil {
		t.Fatalf("failed to sign data: %v", err)
	}

	valid := wallet.Verify(
		w.PublicKey,
		data,
		signature,
	)

	if !valid {
		t.Fatal("expected signature to be valid")
	}
}

func TestModifiedDataFailsVerification(t *testing.T) {
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	originalData := []byte("send 10 coins")

	signature, err := w.Sign(originalData)
	if err != nil {
		t.Fatalf("failed to sign data: %v", err)
	}

	modifiedData := []byte("send 100 coins")

	valid := wallet.Verify(
		w.PublicKey,
		modifiedData,
		signature,
	)

	if valid {
		t.Fatal("expected modified data verification to fail")
	}
}
