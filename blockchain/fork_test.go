package blockchain_test

import (
	"testing"
	"toy-blockchain/block"
	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
	"toy-blockchain/wallet"
)

func mineNextBlockForTest(t *testing.T, blocks []block.Block, txs []transaction.Transaction) block.Block {
	prevBlock := blocks[len(blocks)-1]
	newBlock := block.Block{
		Index:        len(blocks),
		Timestamp:    prevBlock.Timestamp + 60, // Normal block interval (60 seconds)
		Transactions: txs,
		MerkleRoot:   utils.CalculateMerkleRoot(txs),
		PreviousHash: prevBlock.Hash,
		Nonce:        0,
	}

	newBlock.Difficulty = blockchain.CalculateNextDifficulty(blocks)

	mining.MineBlock(&newBlock, newBlock.Difficulty)
	return newBlock
}

func TestValidLongerForkReplacement(t *testing.T) {
	bc := blockchain.NewBlockchain()
	genesis := bc.Blocks[0]

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()
	charlie, _ := wallet.NewWallet()
	david, _ := wallet.NewWallet()
	eve, _ := wallet.NewWallet()

	// 1. Build main chain of length 3
	txs1 := []transaction.Transaction{faucetTx(alice.Address, 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txAliceBob, err := transaction.NewSignedTransaction(alice, bob.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2 := []transaction.Transaction{txAliceBob}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	mainChain := []block.Block{genesis, b1_main, b2_main}
	bc.Blocks = mainChain

	// 2. Build competing fork chain of length 5 starting from genesis
	txs1_fork := []transaction.Transaction{faucetTx(charlie.Address, 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txCharlieDavid, err := transaction.NewSignedTransaction(charlie, david.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2_fork := []transaction.Transaction{txCharlieDavid}
	b2_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork}, txs2_fork)

	txDavidEve, err := transaction.NewSignedTransaction(david, eve.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs3_fork := []transaction.Transaction{txDavidEve}
	b3_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork}, txs3_fork)

	txEveFaucet, err := transaction.NewSignedTransaction(eve, "faucet", 2.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs4_fork := []transaction.Transaction{txEveFaucet}
	b4_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork, b3_fork}, txs4_fork)

	forkChain := []block.Block{genesis, b1_fork, b2_fork, b3_fork, b4_fork}

	// 3. Add fork and resolve
	bc.AddFork(forkChain)
	replaced := bc.ResolveForks()

	if !replaced {
		t.Fatal("Expected active chain to be replaced by the longer fork")
	}

	if len(bc.Blocks) != 5 {
		t.Errorf("Expected chain length 5, got %d", len(bc.Blocks))
	}

	if bc.Blocks[4].Hash != b4_fork.Hash {
		t.Errorf("Expected active chain's tip to be the fork tip %s, got %s", b4_fork.Hash, bc.Blocks[4].Hash)
	}

	if len(bc.Forks) != 0 {
		t.Errorf("Expected resolved fork to be removed from Forks list, remaining: %d", len(bc.Forks))
	}
}

func TestInvalidForkRejected(t *testing.T) {
	bc := blockchain.NewBlockchain()
	genesis := bc.Blocks[0]

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()
	charlie, _ := wallet.NewWallet()
	david, _ := wallet.NewWallet()
	eve, _ := wallet.NewWallet()

	// 1. Build main chain of length 3
	txs1 := []transaction.Transaction{faucetTx(alice.Address, 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txAliceBob, err := transaction.NewSignedTransaction(alice, bob.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2 := []transaction.Transaction{txAliceBob}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	mainChain := []block.Block{genesis, b1_main, b2_main}
	bc.Blocks = mainChain

	// 2. Build invalid competing fork chain of length 5 starting from genesis
	txs1_fork := []transaction.Transaction{faucetTx(charlie.Address, 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txCharlieDavid, err := transaction.NewSignedTransaction(charlie, david.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2_fork := []transaction.Transaction{txCharlieDavid}
	b2_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork}, txs2_fork)

	txDavidEve, err := transaction.NewSignedTransaction(david, eve.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs3_fork := []transaction.Transaction{txDavidEve}
	b3_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork}, txs3_fork)

	txEveFaucet, err := transaction.NewSignedTransaction(eve, "faucet", 2.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs4_fork := []transaction.Transaction{txEveFaucet}
	b4_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork, b3_fork}, txs4_fork)

	// Tamper with a block's transactions list
	b3_fork.Transactions[0].Amount = 999.0

	forkChain := []block.Block{genesis, b1_fork, b2_fork, b3_fork, b4_fork}

	// 3. Add fork and resolve
	bc.AddFork(forkChain)
	replaced := bc.ResolveForks()

	if replaced {
		t.Fatal("Expected fork resolution to reject the invalid fork")
	}

	if len(bc.Blocks) != 3 {
		t.Errorf("Expected chain length to remain 3, got %d", len(bc.Blocks))
	}

	if bc.Blocks[2].Hash != b2_main.Hash {
		t.Errorf("Expected active chain's tip to remain %s, got %s", b2_main.Hash, bc.Blocks[2].Hash)
	}
}

func TestShorterForkIgnored(t *testing.T) {
	bc := blockchain.NewBlockchain()
	genesis := bc.Blocks[0]

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()
	charlie, _ := wallet.NewWallet()
	david, _ := wallet.NewWallet()
	eve, _ := wallet.NewWallet()
	frank, _ := wallet.NewWallet()

	// 1. Build main chain of length 5
	txs1 := []transaction.Transaction{faucetTx(alice.Address, 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txAliceBob, err := transaction.NewSignedTransaction(alice, bob.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2 := []transaction.Transaction{txAliceBob}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	txBobCharlie, err := transaction.NewSignedTransaction(bob, charlie.Address, 2.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs3 := []transaction.Transaction{txBobCharlie}
	b3_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main, b2_main}, txs3)

	txCharlieDavid, err := transaction.NewSignedTransaction(charlie, david.Address, 1.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs4 := []transaction.Transaction{txCharlieDavid}
	b4_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main, b2_main, b3_main}, txs4)

	mainChain := []block.Block{genesis, b1_main, b2_main, b3_main, b4_main}
	bc.Blocks = mainChain

	// 2. Build competing fork chain of length 3 starting from genesis
	txs1_fork := []transaction.Transaction{faucetTx(eve.Address, 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txEveFrank, err := transaction.NewSignedTransaction(eve, frank.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2_fork := []transaction.Transaction{txEveFrank}
	b2_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork}, txs2_fork)

	forkChain := []block.Block{genesis, b1_fork, b2_fork}

	// 3. Add fork and resolve
	bc.AddFork(forkChain)
	replaced := bc.ResolveForks()

	if replaced {
		t.Fatal("Expected fork resolution to ignore the shorter fork")
	}

	if len(bc.Blocks) != 5 {
		t.Errorf("Expected chain length to remain 5, got %d", len(bc.Blocks))
	}

	if bc.Blocks[4].Hash != b4_main.Hash {
		t.Errorf("Expected active chain's tip to remain %s, got %s", b4_main.Hash, bc.Blocks[4].Hash)
	}
}

func TestResolveForksSelectsHeaviestAmongMultipleForks(t *testing.T) {
	bc := blockchain.NewBlockchain()
	genesis := bc.Blocks[0]

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()
	charlie, _ := wallet.NewWallet()
	david, _ := wallet.NewWallet()
	eve, _ := wallet.NewWallet()
	frank, _ := wallet.NewWallet()

	// 1. Build active/main chain of length 3 (Cumulative difficulty: 2)
	txs1 := []transaction.Transaction{faucetTx(alice.Address, 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txAliceBob, err := transaction.NewSignedTransaction(alice, bob.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2 := []transaction.Transaction{txAliceBob}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	activeChain := []block.Block{genesis, b1_main, b2_main}
	bc.Blocks = activeChain

	// 2. Build Fork A of length 5 (Cumulative difficulty: 4)
	txs1_A := []transaction.Transaction{faucetTx(charlie.Address, 20.0)}
	b1_A := mineNextBlockForTest(t, []block.Block{genesis}, txs1_A)

	txCharlieDavid, err := transaction.NewSignedTransaction(charlie, david.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2_A := []transaction.Transaction{txCharlieDavid}
	b2_A := mineNextBlockForTest(t, []block.Block{genesis, b1_A}, txs2_A)

	txDavidEve, err := transaction.NewSignedTransaction(david, eve.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs3_A := []transaction.Transaction{txDavidEve}
	b3_A := mineNextBlockForTest(t, []block.Block{genesis, b1_A, b2_A}, txs3_A)

	txEveFaucet, err := transaction.NewSignedTransaction(eve, "faucet", 2.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs4_A := []transaction.Transaction{txEveFaucet}
	b4_A := mineNextBlockForTest(t, []block.Block{genesis, b1_A, b2_A, b3_A}, txs4_A)

	forkA := []block.Block{genesis, b1_A, b2_A, b3_A, b4_A}

	// 3. Build Fork B of length 4 (Cumulative difficulty: 3)
	txs1_B := []transaction.Transaction{faucetTx(frank.Address, 30.0)}
	b1_B := mineNextBlockForTest(t, []block.Block{genesis}, txs1_B)

	txFrankFaucet, err := transaction.NewSignedTransaction(frank, "faucet", 15.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs2_B := []transaction.Transaction{txFrankFaucet}
	b2_B := mineNextBlockForTest(t, []block.Block{genesis, b1_B}, txs2_B)

	txFaucetCharlie, err := transaction.NewSignedTransaction(frank, charlie.Address, 5.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}
	txs3_B := []transaction.Transaction{txFaucetCharlie}
	b3_B := mineNextBlockForTest(t, []block.Block{genesis, b1_B, b2_B}, txs3_B)

	forkB := []block.Block{genesis, b1_B, b2_B, b3_B}

	// 4. Add forks to blockchain
	// Fork A is index 0, Fork B is index 1
	bc.AddFork(forkA)
	bc.AddFork(forkB)

	// Resolve forks
	replaced := bc.ResolveForks()

	if !replaced {
		t.Fatal("Expected active chain to be replaced by a preferred fork")
	}

	// Fork A (length 5, cumulative diff 4) must be chosen over Fork B (length 4, cumulative diff 3)
	if len(bc.Blocks) != 5 {
		t.Errorf("Expected active chain length to be 5 (Fork A), got %d", len(bc.Blocks))
	}

	if bc.Blocks[4].Hash != b4_A.Hash {
		t.Errorf("Expected active chain's tip to be Fork A's tip %s, got %s", b4_A.Hash, bc.Blocks[4].Hash)
	}

	// Fork A (index 0) was resolved and should be removed. Fork B (index 1) should remain (now at index 0).
	if len(bc.Forks) != 1 {
		t.Errorf("Expected exactly 1 fork to remain in the Forks list, got %d", len(bc.Forks))
	} else if bc.Forks[0][3].Hash != b3_B.Hash {
		t.Errorf("Expected remaining fork in Forks list to be Fork B, but got different hash")
	}
}
