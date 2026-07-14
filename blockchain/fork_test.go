package blockchain_test

import (
	"fmt"
	"testing"
	"toy-blockchain/block"
	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
)

func signTxForForkTest(t *testing.T, sender, receiver string, amount float64) transaction.Transaction {
	tx := transaction.Transaction{
		Sender:   sender,
		Receiver: receiver,
		Amount:   amount,
	}
	if sender != "system" && sender != "faucet" {
		privKey, pubKey, err := utils.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair failed: %v", err)
		}
		data := sender + receiver + fmt.Sprintf("%f", amount)
		sig, err := utils.SignTransaction(data, privKey)
		if err != nil {
			t.Fatalf("SignTransaction failed: %v", err)
		}
		tx.PublicKey = pubKey
		tx.Signature = sig
	}
	return tx
}

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

	// 1. Build main chain of length 3
	txs1 := []transaction.Transaction{signTxForForkTest(t, "faucet", "alice", 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txs2 := []transaction.Transaction{signTxForForkTest(t, "alice", "bob", 5.0)}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	mainChain := []block.Block{genesis, b1_main, b2_main}
	bc.Blocks = mainChain

	// 2. Build competing fork chain of length 5 starting from genesis
	txs1_fork := []transaction.Transaction{signTxForForkTest(t, "faucet", "charlie", 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txs2_fork := []transaction.Transaction{signTxForForkTest(t, "charlie", "david", 10.0)}
	b2_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork}, txs2_fork)

	txs3_fork := []transaction.Transaction{signTxForForkTest(t, "david", "eve", 5.0)}
	b3_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork}, txs3_fork)

	txs4_fork := []transaction.Transaction{signTxForForkTest(t, "eve", "faucet", 2.0)}
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

	// 1. Build main chain of length 3
	txs1 := []transaction.Transaction{signTxForForkTest(t, "faucet", "alice", 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txs2 := []transaction.Transaction{signTxForForkTest(t, "alice", "bob", 5.0)}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	mainChain := []block.Block{genesis, b1_main, b2_main}
	bc.Blocks = mainChain

	// 2. Build invalid competing fork chain of length 5 starting from genesis
	txs1_fork := []transaction.Transaction{signTxForForkTest(t, "faucet", "charlie", 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txs2_fork := []transaction.Transaction{signTxForForkTest(t, "charlie", "david", 10.0)}
	b2_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork}, txs2_fork)

	txs3_fork := []transaction.Transaction{signTxForForkTest(t, "david", "eve", 5.0)}
	b3_fork := mineNextBlockForTest(t, []block.Block{genesis, b1_fork, b2_fork}, txs3_fork)

	txs4_fork := []transaction.Transaction{signTxForForkTest(t, "eve", "faucet", 2.0)}
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

	// 1. Build main chain of length 5
	txs1 := []transaction.Transaction{signTxForForkTest(t, "faucet", "alice", 10.0)}
	b1_main := mineNextBlockForTest(t, []block.Block{genesis}, txs1)

	txs2 := []transaction.Transaction{signTxForForkTest(t, "alice", "bob", 5.0)}
	b2_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main}, txs2)

	txs3 := []transaction.Transaction{signTxForForkTest(t, "bob", "charlie", 2.0)}
	b3_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main, b2_main}, txs3)

	txs4 := []transaction.Transaction{signTxForForkTest(t, "charlie", "david", 1.0)}
	b4_main := mineNextBlockForTest(t, []block.Block{genesis, b1_main, b2_main, b3_main}, txs4)

	mainChain := []block.Block{genesis, b1_main, b2_main, b3_main, b4_main}
	bc.Blocks = mainChain

	// 2. Build competing fork chain of length 3 starting from genesis
	txs1_fork := []transaction.Transaction{signTxForForkTest(t, "faucet", "eve", 20.0)}
	b1_fork := mineNextBlockForTest(t, []block.Block{genesis}, txs1_fork)

	txs2_fork := []transaction.Transaction{signTxForForkTest(t, "eve", "frank", 10.0)}
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
