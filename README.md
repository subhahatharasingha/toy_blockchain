# Toy Blockchain and Ledger Simulator

A modular Command Line Interface (CLI) blockchain simulator developed from scratch using **Go (Golang)**.

This project demonstrates the core concepts of blockchain technology including cryptographic hashing, Proof-of-Work mining, transaction validation, ledger management, blockchain integrity verification, persistent storage, and automated testing.

The system is designed as an educational blockchain implementation to provide practical understanding of how blockchain networks operate internally.

---

# Objectives

The objectives of this project are:

* Understand blockchain architecture and design principles.
* Implement a blockchain without using external blockchain frameworks.
* Build a transaction verification mechanism.
* Implement Proof-of-Work (PoW) mining.
* Maintain a ledger for balance tracking.
* Detect blockchain tampering using cryptographic hashes.
* Persist blockchain data using JSON storage.
* Develop a modular CLI application using Go.
* Apply software engineering practices such as testing, validation, and documentation.

---

# Technologies Used

* Go (Golang)
* SHA-256 Cryptographic Hashing
* JSON Serialization
* Command Line Interface (CLI)
* Go Standard Library
* Go Testing Framework

---

# Features

## Blockchain Features

* Genesis block creation
* Automatic block indexing
* Previous hash linking
* Timestamp generation
* Nonce generation
* Blockchain integrity validation
* Blockchain printing and inspection

## Hashing Features

* SHA-256 cryptographic hashing
* Deterministic hash generation
* Tamper detection through hash verification
* Hash recalculation validation

## Mining Features

* Proof-of-Work implementation
* Configurable mining difficulty
* Nonce-based mining
* Mining duration measurement
* Difficulty target verification

## Transaction Features

* Pending transaction pool (mempool)
* Transaction validation
* Positive amount verification
* Self-transfer prevention
* Overspending prevention
* Double-spending prevention
* Sender balance verification

## Ledger Features

* Automatic balance calculation
* Account balance lookup
* Transaction application to ledger
* Historical balance reconstruction from blockchain

## Storage Features

* Blockchain persistence using JSON
* Automatic blockchain loading
* Automatic blockchain saving
* Pending transaction persistence

## Testing Features

* Hash generation tests
* Mining tests
* Blockchain validation tests
* Blockchain tampering tests
* Transaction rejection tests
* Double-spending prevention tests
* Balance calculation tests

---

# Blockchain Architecture

User

↓

CLI Commands (main.go)

↓

Blockchain Package

├── Transaction Validation

├── Pending Transaction Pool

├── Block Creation

├── Blockchain Validation

↓

Mining Package

↓

Proof-of-Work

↓

Ledger Package

↓

Storage Package (JSON)

---

# Project Structure

```text
toy-blockchain/

├── block/
│   └── block.go

├── blockchain/
│   ├── blockchain.go
│   └── blockchain_test.go

├── ledger/
│   └── ledger.go

├── mining/
│   ├── mining.go
│   └── mining_test.go

├── storage/
│   └── storage.go

├── transaction/
│   └── transaction.go

├── utils/
│   ├── hash.go
│   └── hash_test.go

├── blockchain.json
├── main.go
├── go.mod
├── go.sum
└── README.md
```

---

# Package Description

## block

Contains the Block data structure.

## transaction

Defines the Transaction model used throughout the blockchain.

## blockchain

Handles blockchain management, transaction validation, block creation, and chain verification.

## mining

Implements the Proof-of-Work mining algorithm.

## ledger

Tracks balances and applies transactions.

## utils

Provides SHA-256 hash generation functionality.

## storage

Handles blockchain persistence using JSON files.

## main.go

Implements the CLI interface and command handling.

---

# Data Structures

## Block

Each block contains:

* Index
* Timestamp
* Transactions
* PreviousHash
* Hash
* Nonce

## Transaction

Each transaction contains:

* Sender
* Receiver
* Amount

## Blockchain

Maintains:

* Blocks
* PendingTransactions

## Ledger

Maintains:

* Balances (map[string]float64)

---

# Requirements

* Go 1.22 or newer

Verify installation:

```bash
go version
```

---

# Installation

Clone the repository:

```bash
git clone <repository-url>
```

Navigate to the project folder:

```bash
cd toy-blockchain
```

Build the project:

```bash
go build -o toy-blockchain main.go
```

Run tests:

```bash
go test -v ./...
```

---

# Configuration Options

| Flag        | Description                    | Default         |
| ----------- | ------------------------------ | --------------- |
| -difficulty | Mining difficulty              | 4               |
| -file       | Blockchain JSON file           | blockchain.json |
| -blocksize  | Maximum transactions per block | 10              |

Example:

```bash
./toy-blockchain -difficulty 3 -blocksize 5 mine
```

---

# CLI Commands


## Faucet/System Account

To introduce initial coins into the blockchain, the system supports a special account:


## Add Transaction

```bash
./toy-blockchain addtx <sender> <receiver> <amount>
```

Example:

```bash
./toy-blockchain addtx faucet alice 100
```

---

## Mine Pending Transactions

```bash
./toy-blockchain mine
```

---

## Print Blockchain

```bash
./toy-blockchain print
```

---

## Get Account Balance

```bash
./toy-blockchain balance <user>
```

Example:

```bash
./toy-blockchain balance alice
```

---

## Validate Blockchain

```bash
./toy-blockchain validate
```

---

## Save Blockchain

```bash
./toy-blockchain save
```

---

# Proof-of-Work Algorithm

The mining process follows these steps:

1. Create a candidate block.
2. Initialize nonce to zero.
3. Calculate the block hash using SHA-256.
4. Check whether the hash satisfies the difficulty target.
5. If invalid, increment nonce.
6. Repeat until a valid hash is found.
7. Store the nonce and hash.
8. Append the block to the blockchain.

---

# Transaction Validation

Every transaction is verified before entering the pending transaction pool.

Validation rules:

* Amount must be greater than zero.
* Sender cannot be empty.
* Receiver cannot be empty.
* Sender and receiver cannot be the same.
* Sender must have sufficient balance.
* Pending transactions cannot exceed available balance.
* Double-spending is prevented.
* Faucet/System accounts may create initial coins.

---

# Blockchain Validation

The validator verifies:

* Genesis block correctness
* Block index sequence
* Previous hash links
* SHA-256 hash integrity
* Proof-of-Work difficulty
* Timestamp ordering

Any unauthorized modification invalidates the blockchain.

---

# Example Workflow

Create initial funds:

```bash
./toy-blockchain addtx faucet alice 100
```

Mine the transaction:

```bash
./toy-blockchain mine
```

Transfer coins:

```bash
./toy-blockchain addtx alice bob 40
```

Mine the transfer:

```bash
./toy-blockchain mine
```

Check balance:

```bash
./toy-blockchain balance alice
./toy-blockchain balance bob
```

Validate blockchain:

```bash
./toy-blockchain validate
```

Print blockchain:

```bash
./toy-blockchain print
```

---

# Sample Output

```text
Transaction added successfully.

Mining block 1...
Nonce found: 2315
Hash: 0000a7d7e12f...

Block mined successfully.

Balance of alice: 60.000000
Balance of bob: 40.000000

Blockchain is VALID.
```

---

# Unit Testing

Automated tests verify:

* SHA-256 hash generation
* Proof-of-Work mining
* Blockchain validation
* Tamper detection
* Transaction validation
* Overspending prevention
* Double-spending prevention
* Balance calculation

Run all tests:

```bash
go test ./...
```

---

# Security Features

* SHA-256 cryptographic hashing
* Proof-of-Work mining
* Block tamper detection
* Transaction validation
* Balance verification
* Double-spend prevention
* Persistent blockchain storage

---

# Running with Docker Compose

An additional way to run the 3-node network cluster is using Docker Compose.

### Start the Cluster
To build the image and start the 3-node cluster:
```bash
docker compose up --build
```

This starts:
* **Node A:** http://localhost:8001 (Seed Node)
* **Node B:** http://localhost:8002 (Connects to Node A, auto-discovers Node C)
* **Node C:** http://localhost:8003 (Connects to Node A, auto-discovers Node B)

### Status Example
From the host machine, you can check the status of a node using:
```bash
# On Windows PowerShell
Invoke-RestMethod http://127.0.0.1:8001/status

# Or using curl
curl http://localhost:8001/status
```

### Stop the Cluster
To stop the cluster:
```bash
docker compose down
```

### Reset Cluster Database State
To stop the cluster and delete the persistent database volumes:
```bash
docker compose down -v
```

### Network Communication Notes
* Internally within the Docker network, nodes communicate with each other using their Docker service names (e.g., `http://node-a:8001`).
* Externally from your host machine, you access the nodes through `localhost` on the mapped ports (`8001`, `8002`, `8003`).


---





