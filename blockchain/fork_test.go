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
