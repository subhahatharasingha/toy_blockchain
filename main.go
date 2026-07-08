package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/storage"
	"toy-blockchain/transaction"
)

var (
	difficultyFlag = flag.Int("difficulty", 4, "Mining difficulty (number of leading zeros in block hash hex)")
	fileFlag       = flag.String("file", "blockchain.json", "Path to the blockchain database file")
	blockSizeFlag  = flag.Int("blocksize", 10, "Maximum number of transactions per block")
)

func printUsage() {
	fmt.Println("Toy Blockchain and Ledger Simulator CLI")
	fmt.Println("\nUsage:")
	fmt.Println("  toy-blockchain [flags] <command> [arguments]")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
	fmt.Println("\nCommands:")
	fmt.Println("  addtx <sender> <receiver> <amount> - Add a new pending transaction")
	fmt.Println("  mine                               - Mine a block with pending transactions")
	fmt.Println("  print                              - Print the full blockchain details")
	fmt.Println("  balance <user>                     - Get the balance of a specific user")
	fmt.Println("  validate                           - Validate the full chain integrity")
	fmt.Println("  save                               - Save current memory state (utility command)")
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		return
	}

	// Load the blockchain from disk at the configured path
	bc, err := storage.Load(*fileFlag)
	if err != nil {
		fmt.Printf("Fatal error loading blockchain from '%s': %v\n", *fileFlag, err)
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "addtx":
		addTransaction(bc, args)
	case "mine":
		mineBlock(bc)
	case "print":
		bc.PrintChain()
	case "balance":
		getBalance(bc, args)
	case "validate":
		validateChain(bc)
	case "save":
		// Explicit save command if the user wants to ensure write is flushed
		err := storage.Save(bc, *fileFlag)
		if err != nil {
			fmt.Printf("Error saving blockchain: %v\n", err)
		} else {
			fmt.Printf("Blockchain state saved successfully to '%s'.\n", *fileFlag)
		}
	default:
		fmt.Printf("Unknown command '%s'\n\n", command)
		printUsage()
	}
}

func addTransaction(bc *blockchain.Blockchain, args []string) {
	if len(args) < 4 {
		fmt.Println("Error: Missing parameters for addtx command.")
		fmt.Println("Usage: addtx <sender> <receiver> <amount>")
		return
	}

	sender := args[1]
	receiver := args[2]
	amountStr := args[3]

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		fmt.Printf("Error: invalid amount '%s'\n", amountStr)
		return
	}

	tx := transaction.Transaction{
		Sender:   sender,
		Receiver: receiver,
		Amount:   amount,
	}

	// AddTransaction performs verification internally
	err = bc.AddTransaction(tx)
	if err != nil {
		fmt.Printf("Transaction rejected: %v\n", err)
		return
	}

	// Persist the transaction immediately so it is not lost when process exits
	err = storage.Save(bc, *fileFlag)
	if err != nil {
		fmt.Printf("Transaction added to pool, but failed to persist blockchain state: %v\n", err)
		return
	}

	fmt.Printf("Transaction added successfully: %s sends %f to %s\n", sender, amount, receiver)
}

func mineBlock(bc *blockchain.Blockchain) {
	blockData, err := bc.CreatePendingBlock(*blockSizeFlag)
	if err != nil {
		fmt.Printf("Mining aborted: %v\n", err)
		return
	}

	// Set mining timestamp
	blockData.Timestamp = time.Now().Unix()

	fmt.Printf("Mining block %d with %d transactions (difficulty target: %d)...\n",
		blockData.Index, len(blockData.Transactions), *difficultyFlag)

	nonce, elapsed := mining.MineBlock(&blockData, *difficultyFlag)

	bc.AddMinedBlock(blockData)

	err = storage.Save(bc, *fileFlag)
	if err != nil {
		fmt.Printf("Block mined successfully, but failed to save blockchain to disk: %v\n", err)
		return
	}

	fmt.Printf("Block %d mined successfully!\n", blockData.Index)
	fmt.Printf("  Nonce found:  %d\n", nonce)
	fmt.Printf("  Block Hash:   %s\n", blockData.Hash)
	fmt.Printf("  Time elapsed: %s\n", elapsed)
}

func getBalance(bc *blockchain.Blockchain, args []string) {
	if len(args) < 2 {
		fmt.Println("Error: Missing user parameter for balance command.")
		fmt.Println("Usage: balance <user>")
		return
	}

	user := args[1]
	balance := bc.GetBalance(user)
	fmt.Printf("Balance of '%s': %f\n", user, balance)
}

func validateChain(bc *blockchain.Blockchain) {
	valid, index, err := bc.Validate(*difficultyFlag)
	if valid {
		fmt.Println("Blockchain is VALID (integrity check passed).")
	} else {
		fmt.Println("Blockchain is INVALID!")
		fmt.Printf("  First offending block index: %d\n", index)
		fmt.Printf("  Reason: %v\n", err)
	}
}