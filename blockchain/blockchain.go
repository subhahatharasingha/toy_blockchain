package blockchain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"toy-blockchain/block"
	"toy-blockchain/ledger"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
	"toy-blockchain/wallet"
)

// GenesisTimestamp is a fixed Unix timestamp to make the Genesis block completely deterministic.
const GenesisTimestamp = 1719830400 // July 1, 2024, 00:00:00 UTC

// Blockchain maintains the list of validated blocks and the pool of pending transactions.
type Blockchain struct {
	Blocks              []block.Block             `json:"blocks"`
	PendingTransactions []transaction.Transaction `json:"pending_transactions"`
	Forks               [][]block.Block           `json:"forks"`
}

// NewBlockchain creates a blockchain containing only the genesis block.
func NewBlockchain() *Blockchain {
	bc := &Blockchain{
		Blocks:              []block.Block{},
		PendingTransactions: []transaction.Transaction{},
		Forks:               [][]block.Block{},
	}

	genesis := createGenesisBlock()
	bc.Blocks = append(bc.Blocks, genesis)

	return bc
}

// createGenesisBlock creates the first block in the blockchain.
func createGenesisBlock() block.Block {
	genesis := block.Block{
		Index:        0,
		Timestamp:    GenesisTimestamp,
		Transactions: []transaction.Transaction{},
		MerkleRoot:   "",
		PreviousHash: "0",
		Nonce:        0,
		Difficulty:   0,
	}

	genesis.Hash = utils.CalculateHash(genesis)

	return genesis
}

// VerifyTransaction checks if a transaction is valid.
func (bc *Blockchain) VerifyTransaction(tx transaction.Transaction) error {
	if tx.Amount <= 0 || math.IsNaN(tx.Amount) || math.IsInf(tx.Amount, 0) {
		return errors.New("invalid transaction amount")
	}
	if tx.Sender == "" {
		return errors.New("sender name cannot be empty")
	}
	if tx.Receiver == "" {
		return errors.New("receiver name cannot be empty")
	}
	if tx.Sender == tx.Receiver {
		return errors.New("sender and receiver cannot be the same account")
	}

	// Validate transaction ID uniqueness
	if tx.ID != "" {
		for _, pTx := range bc.PendingTransactions {
			if pTx.ID == tx.ID {
				return errors.New("transaction already pending")
			}
		}
		for _, b := range bc.Blocks {
			for _, bTx := range b.Transactions {
				if bTx.ID == tx.ID {
					return errors.New("transaction already mined")
				}
			}
		}
	}

	// Verify digital signature and ID
	// System and faucet accounts are allowed without signatures
	if tx.Sender != "system" && tx.Sender != "faucet" {
		if tx.PublicKey == "" || tx.Signature == "" {
			return errors.New("invalid transaction signature")
		}

		// Decode public key
		pubKeyBytes, err := base64.StdEncoding.DecodeString(tx.PublicKey)
		if err != nil || len(pubKeyBytes) != 32 {
			return errors.New("invalid transaction signature")
		}

		// Decode signature
		sigBytes, err := base64.StdEncoding.DecodeString(tx.Signature)
		if err != nil || len(sigBytes) != 64 {
			return errors.New("invalid transaction signature")
		}

		// Verify signature bytes against signing payload
		signingBytes, err := tx.SigningBytes()
		if err != nil || !wallet.Verify(pubKeyBytes, signingBytes, sigBytes) {
			return errors.New("invalid transaction signature")
		}

		// If signature is mathematically valid, then check if ID is correct
		if tx.ID == "" || !tx.VerifyID() {
			return errors.New("invalid transaction ID")
		}
	}

	if tx.Sender == "system" || tx.Sender == "faucet" {
		return nil
	}

	// Calculate current balance of the sender from all mined blocks
	balance := bc.GetBalance(tx.Sender)

	// Calculate the total amount the sender has already committed in pending transactions
	var pendingSpent float64
	for _, pendingTx := range bc.PendingTransactions {
		if pendingTx.Sender == tx.Sender {
			pendingSpent += pendingTx.Amount
		}
	}

	// Check if sender has enough funds remaining
	if balance-pendingSpent < tx.Amount {
		return fmt.Errorf("insufficient balance: sender '%s' has %f (with %f pending spent), trying to send %f",
			tx.Sender, balance, pendingSpent, tx.Amount)
	}

	return nil
}

// AddTransaction adds a transaction to the pending transaction pool after validating it.
func (bc *Blockchain) AddTransaction(tx transaction.Transaction) error {
	if tx.ID == "" {
		id, err := tx.CalculateID()
		if err == nil {
			tx.ID = id
		}
	}
	if err := bc.VerifyTransaction(tx); err != nil {
		return err
	}
	bc.PendingTransactions = append(bc.PendingTransactions, tx)
	return nil
}

// GetLatestBlock returns the newest block.
func (bc *Blockchain) GetLatestBlock() block.Block {
	if len(bc.Blocks) == 0 {
		return block.Block{}
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// CreatePendingBlock creates a new candidate block from pending transactions.
// It limits the block to maxTx transactions if maxTx is positive.
func (bc *Blockchain) CreatePendingBlock(maxTx int) (block.Block, error) {
	if len(bc.PendingTransactions) == 0 {
		return block.Block{}, errors.New("no pending transactions to mine")
	}

	limit := len(bc.PendingTransactions)
	if maxTx > 0 && maxTx < limit {
		limit = maxTx
	}

	// Copy transactions to include in this block
	txsToInclude := make([]transaction.Transaction, limit)
	copy(txsToInclude, bc.PendingTransactions[:limit])

	newBlock := block.Block{
		Index:        len(bc.Blocks),
		Timestamp:    0, // Will be set before mining
		Transactions: txsToInclude,
		MerkleRoot:   utils.CalculateMerkleRoot(txsToInclude),
		PreviousHash: bc.GetLatestBlock().Hash,
		Nonce:        0,
		Difficulty:   CalculateNextDifficulty(bc.Blocks),
		Hash:         "",
	}

	return newBlock, nil
}

// AddMinedBlock appends a mined block to the chain and removes its transactions from the pending pool.
func (bc *Blockchain) AddMinedBlock(b block.Block) {
	bc.Blocks = append(bc.Blocks, b)

	// Remove the mined transactions from the pending pool by matching ID
	minedIDs := make(map[string]struct{})
	for _, tx := range b.Transactions {
		minedIDs[tx.ID] = struct{}{}
	}

	newPending := make([]transaction.Transaction, 0, len(bc.PendingTransactions))
	for _, tx := range bc.PendingTransactions {
		if _, mined := minedIDs[tx.ID]; !mined {
			newPending = append(newPending, tx)
		}
	}
	bc.PendingTransactions = newPending
}

// VerifyBlock checks if a single block is valid to be appended to the current chain.
func (bc *Blockchain) VerifyBlock(b block.Block) error {
	latest := bc.GetLatestBlock()

	// 1. Check index sequentiality
	if b.Index != latest.Index+1 {
		return fmt.Errorf("block index %d is out of sequence, expected %d", b.Index, latest.Index+1)
	}

	// 2. Previous hash link must match
	if b.PreviousHash != latest.Hash {
		return fmt.Errorf("previous hash mismatch: block stores previous hash '%s', but latest block has hash '%s'", b.PreviousHash, latest.Hash)
	}

	// 3. Check difficulty adjustment sequence
	expectedDifficulty := CalculateNextDifficulty(bc.Blocks)
	if b.Difficulty != expectedDifficulty {
		return fmt.Errorf("invalid difficulty adjustment: expected %d, got %d", expectedDifficulty, b.Difficulty)
	}

	// 4. Stored hash must match recalculated hash
	recalculatedHash := utils.CalculateHash(b)
	if b.Hash != recalculatedHash {
		return fmt.Errorf("block hash mismatch: block stores hash '%s', but recalculated hash is '%s'", b.Hash, recalculatedHash)
	}

	// 5. Proof of work must be valid
	blockTarget := strings.Repeat("0", b.Difficulty)
	if len(b.Hash) < b.Difficulty || b.Hash[:b.Difficulty] != blockTarget {
		return fmt.Errorf("block hash '%s' does not satisfy difficulty target %d", b.Hash, b.Difficulty)
	}

	// 6. Check timestamp consistency (chronological order)
	if b.Timestamp < latest.Timestamp {
		return fmt.Errorf("block timestamp %d is earlier than latest block timestamp %d", b.Timestamp, latest.Timestamp)
	}

	// 7. Check merkle root
	calculatedMerkleRoot := utils.CalculateMerkleRoot(b.Transactions)
	if b.MerkleRoot != calculatedMerkleRoot {
		return fmt.Errorf("invalid merkle root")
	}

	// 8. Verify transaction signatures across all block transactions (skip faucet/system)
	for _, tx := range b.Transactions {
		if tx.Sender == "faucet" || tx.Sender == "system" {
			continue
		}
		if tx.ID == "" || !tx.VerifyID() {
			return fmt.Errorf("invalid transaction ID")
		}
		if tx.PublicKey == "" || tx.Signature == "" {
			return fmt.Errorf("missing signature fields for transaction")
		}
		if !tx.VerifySignature() {
			return fmt.Errorf("invalid transaction signature")
		}
	}

	return nil
}

// PrintChain prints details of every block in the chain to the console.
func (bc *Blockchain) PrintChain() {
	fmt.Printf("Blocks: %d\n", len(bc.Blocks))
	fmt.Printf("Forks available: %d\n", len(bc.Forks))
	for _, b := range bc.Blocks {
		fmt.Println("-----------------------------------")
		fmt.Printf("Index:        %d\n", b.Index)
		fmt.Printf("Timestamp:    %d (%s)\n", b.Timestamp, fmt.Sprint(b.Timestamp))
		fmt.Printf("Difficulty:   %d\n", b.Difficulty)
		fmt.Printf("PreviousHash: %s\n", b.PreviousHash)
		fmt.Printf("Hash:         %s\n", b.Hash)
		fmt.Printf("Merkle Root:  %s\n", b.MerkleRoot)
		fmt.Printf("Nonce:        %d\n", b.Nonce)
		fmt.Printf("Transactions: %d\n", len(b.Transactions))
		for j, tx := range b.Transactions {
			fmt.Printf("  [%d] %s -> %s: %f\n", j, tx.Sender, tx.Receiver, tx.Amount)
		}
	}
	fmt.Println("-----------------------------------")
}

// Validate checks the integrity of the blockchain.
func (bc *Blockchain) Validate(difficulty int) (bool, int, error) {
	if len(bc.Blocks) == 0 {
		return false, -1, errors.New("blockchain is empty")
	}

	// Verify Genesis Block
	genesis := bc.Blocks[0]
	if genesis.Index != 0 {
		return false, 0, errors.New("genesis block index must be 0")
	}
	if genesis.PreviousHash != "0" {
		return false, 0, fmt.Errorf("genesis block previous hash must be '0', got '%s'", genesis.PreviousHash)
	}
	calculatedGenesisMerkleRoot := utils.CalculateMerkleRoot(genesis.Transactions)
	if genesis.MerkleRoot != calculatedGenesisMerkleRoot {
		return false, 0, errors.New("invalid merkle root")
	}
	expectedGenesisHash := utils.CalculateHash(genesis)
	if genesis.Hash != expectedGenesisHash {
		return false, 0, fmt.Errorf("genesis block hash is invalid: stored '%s', recalculated '%s'", genesis.Hash, expectedGenesisHash)
	}

	// Verify Subsequent Blocks
	for i := 1; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		previous := bc.Blocks[i-1]

		// 0. Check difficulty adjustment sequence
		expectedDifficulty := CalculateNextDifficulty(bc.Blocks[:i])
		if current.Difficulty != expectedDifficulty {
			return false, i, errors.New("invalid difficulty adjustment")
		}

		// 0. Check merkle root
		calculatedMerkleRoot := utils.CalculateMerkleRoot(current.Transactions)
		if current.MerkleRoot != calculatedMerkleRoot {
			return false, i, errors.New("invalid merkle root")
		}

		// 1. Check index sequentiality
		if current.Index != i {
			return false, i, fmt.Errorf("block index %d is out of sequence, expected %d", current.Index, i)
		}

		// 2. Previous hash link must match
		if current.PreviousHash != previous.Hash {
			return false, i, fmt.Errorf("previous hash mismatch: block %d stores previous hash '%s', but block %d has hash '%s'", i, current.PreviousHash, i-1, previous.Hash)
		}

		// 3. Stored hash must match recalculated hash
		recalculatedHash := utils.CalculateHash(current)
		if current.Hash != recalculatedHash {
			return false, i, fmt.Errorf("block hash mismatch: block %d stores hash '%s', but recalculated hash is '%s'", i, current.Hash, recalculatedHash)
		}

		// 4. Proof of work must be valid
		blockTarget := strings.Repeat("0", current.Difficulty)
		if len(current.Hash) < current.Difficulty || current.Hash[:current.Difficulty] != blockTarget {
			return false, i, fmt.Errorf("block hash '%s' does not satisfy difficulty target %d (must start with '%s')", current.Hash, current.Difficulty, blockTarget)
		}

		// 5. Check timestamp consistency (chronological order)
		if current.Timestamp < previous.Timestamp {
			return false, i, fmt.Errorf("block timestamp %d is earlier than previous block timestamp %d", current.Timestamp, previous.Timestamp)
		}
	}

	// 6. Verify transaction signatures across all blocks (skip faucet/system)
	for i := 0; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		for _, tx := range current.Transactions {
			if tx.Sender == "faucet" || tx.Sender == "system" {
				continue
			}
			if tx.PublicKey == "" || tx.Signature == "" {
				return false, i, fmt.Errorf("missing signature fields for transaction")
			}
			if !tx.VerifySignature() {
				return false, i, fmt.Errorf("invalid transaction signature")
			}
		}
	}

	return true, -1, nil
}

// GetBalance calculates a user's balance from the blockchain by traversing all mined transactions.
func (bc *Blockchain) GetBalance(user string) float64 {
	l := ledger.NewLedger()
	for _, b := range bc.Blocks {
		for _, tx := range b.Transactions {
			l.ApplyTransaction(tx.Sender, tx.Receiver, tx.Amount)
		}
	}
	return l.GetBalance(user)
}
