package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"toy-blockchain/blockchain"
	"toy-blockchain/mining"
	"toy-blockchain/node"
	"toy-blockchain/storage"
	"toy-blockchain/transaction"
	"toy-blockchain/wallet"
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
	fmt.Println("  node --id <id> --host <host> --port <port> - Start a networked blockchain node")
	fmt.Println("  addtx <sender> <receiver> <amount> - Add a new pending transaction")
	fmt.Println("  mine                               - Mine a block with pending transactions")
	fmt.Println("  print                              - Print the full blockchain details")
	fmt.Println("  balance <user>                     - Get the balance of a specific user")
	fmt.Println("  validate                           - Validate the full chain integrity")
	fmt.Println("  resolve                            - Resolve forks and choose the longest valid chain")
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

	command := args[0]
	if command == "node" {
		runNode(args)
		return
	}

	// Load the blockchain from disk at the configured path
	bc, err := storage.Load(*fileFlag)
	if err != nil {
		fmt.Printf("Fatal error loading blockchain from '%s': %v\n", *fileFlag, err)
		os.Exit(1)
	}

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
	case "resolve":
		resolveForks(bc)
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

	if sender != "system" && sender != "faucet" {
		fmt.Println("Error: normal-user transactions cannot yet be signed via CLI (no persistent wallet support).")
		return
	}

	var tx transaction.Transaction
	if sender == "faucet" {
		tx, err = transaction.NewSignedSpecialTransaction("faucet", wallet.FaucetPrivateKey, wallet.FaucetPublicKey, receiver, amount)
	} else {
		tx, err = transaction.NewSignedSpecialTransaction("system", wallet.SystemPrivateKey, wallet.SystemPublicKey, receiver, amount)
	}

	if err != nil {
		fmt.Printf("Failed to sign transaction: %v\n", err)
		return
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

	fmt.Printf("Mining block %d with %d transactions...\n", blockData.Index, len(blockData.Transactions))
	fmt.Printf("Difficulty target: %d\n", blockData.Difficulty)

	workers := 4

	nonce, elapsed := mining.ConcurrentMineBlock(
		&blockData,
		blockData.Difficulty,
		workers,
	)

	bc.AddMinedBlock(blockData)

	err = storage.Save(bc, *fileFlag)
	if err != nil {
		fmt.Printf("Block mined successfully, but failed to save blockchain to disk: %v\n", err)
		return
	}

	fmt.Printf("Block %d mined successfully!\n", blockData.Index)
	fmt.Printf("  Workers used: %d\n", workers)
	fmt.Printf("  Nonce found:  %d\n", nonce)
	fmt.Printf("  Block Hash:   %s\n", blockData.Hash)
	fmt.Printf("  Time elapsed: %s\n", elapsed)
	fmt.Printf("Difficulty used: %d\n", blockData.Difficulty)
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

func resolveForks(bc *blockchain.Blockchain) {
	replaced := bc.ResolveForks()
	err := storage.Save(bc, *fileFlag)
	if err != nil {
		fmt.Printf("Error saving blockchain: %v\n", err)
		return
	}

	if replaced {
		fmt.Println("Fork resolved. Replaced chain with longer valid chain.")
	} else {
		fmt.Println("No longer valid fork found.")
	}
}

func runNode(args []string) {
	nodeCmd := flag.NewFlagSet("node", flag.ContinueOnError)
	idFlag := nodeCmd.String("id", "", "Node ID")
	hostFlag := nodeCmd.String("host", "", "Node Host")
	portFlag := nodeCmd.Int("port", 0, "Node Port")
	fileFlagSub := nodeCmd.String("file", "", "Path to the blockchain database file")
	peersFlag := nodeCmd.String("peers", "", "Comma-separated list of peer URLs")

	if err := nodeCmd.Parse(args[1:]); err != nil {
		os.Exit(1)
	}

	id := *idFlag
	host := *hostFlag
	port := *portFlag
	dbPath := *fileFlagSub

	if id == "" {
		fmt.Fprintln(os.Stderr, "Error: Node ID cannot be empty.")
		os.Exit(1)
	}
	if host == "" {
		fmt.Fprintln(os.Stderr, "Error: Node host cannot be empty.")
		os.Exit(1)
	}
	if port <= 0 || port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: Invalid port number: %d. Must be between 1 and 65535.\n", port)
		os.Exit(1)
	}

	// Support persistence loading
	var bc *blockchain.Blockchain
	if dbPath != "" {
		var err error
		bc, err = storage.Load(dbPath)
		if err != nil {
			fmt.Printf("Database file '%s' not found or corrupt, creating new blockchain: %v\n", dbPath, err)
			bc = blockchain.NewBlockchain()
			// Create directory if not exists
			dir := dbPath
			if lastIdx := strings.LastIndex(dbPath, "/"); lastIdx != -1 {
				dir = dbPath[:lastIdx]
			} else if lastIdx := strings.LastIndex(dbPath, "\\"); lastIdx != -1 {
				dir = dbPath[:lastIdx]
			}
			if dir != dbPath {
				_ = os.MkdirAll(dir, 0755)
			}
			_ = storage.Save(bc, dbPath)
		}
	} else {
		bc = blockchain.NewBlockchain()
	}

	n := node.NewNode(id, host, port, bc)
	n.DbPath = dbPath

	// Parse peers list
	if *peersFlag != "" {
		peerList := strings.Split(*peersFlag, ",")
		for _, rawPeer := range peerList {
			rawPeer = strings.TrimSpace(rawPeer)
			if rawPeer == "" {
				continue
			}

			cleanPeer := rawPeer
			cleanPeer = strings.TrimPrefix(cleanPeer, "http://")
			cleanPeer = strings.TrimPrefix(cleanPeer, "https://")

			peerHost, peerPortStr, err := net.SplitHostPort(cleanPeer)
			if err != nil {
				parts := strings.Split(cleanPeer, ":")
				if len(parts) == 2 {
					peerHost = parts[0]
					peerPortStr = parts[1]
				} else {
					fmt.Fprintf(os.Stderr, "Warning: invalid peer address format '%s': %v\n", rawPeer, err)
					continue
				}
			}

			var peerPort int
			_, err = fmt.Sscanf(peerPortStr, "%d", &peerPort)
			if err != nil || peerPort <= 0 || peerPort > 65535 {
				fmt.Fprintf(os.Stderr, "Warning: invalid peer port '%s'\n", peerPortStr)
				continue
			}

			placeholderID := fmt.Sprintf("peer-%s-%d", peerHost, peerPort)
			err = n.AddPeer(node.Peer{
				ID:   placeholderID,
				Host: peerHost,
				Port: peerPort,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to add peer '%s': %v\n", rawPeer, err)
			}
		}
	}

	if err := n.StartServer(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting node server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Node ID:          %s\n", n.ID)
	fmt.Printf("Host:             %s\n", n.Host)
	fmt.Printf("Port:             %d\n", n.Port)
	fmt.Printf("Health Endpoint:  http://%s:%d/health\n", n.Host, n.Port)
	fmt.Printf("Chain Endpoint:   http://%s:%d/chain\n", n.Host, n.Port)
	fmt.Println("Node is running. Press Ctrl+C to terminate.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	fmt.Println("\nShutting down node server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := n.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error shutting down node server: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Node server stopped.")
}
