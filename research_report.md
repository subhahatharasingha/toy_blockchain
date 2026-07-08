# Research Report: Toy Blockchain and Ledger Simulator

**Author:** Software Engineering Intern Candidate (Backend, Go)  
**Date:** July 2026  
**Language/Version:** Go 1.22+  

---

## 1. Design Write-up

### 1.1 Hashing Scheme
To ensure cryptographic integrity and deterministic block identification, a SHA-256 hashing scheme is implemented in [utils/hash.go](file:///c:/intern/toy-blockchain/utils/hash.go). Hashing a block twice must always yield the exact same byte sequence.

#### Fields Feeding the Hash
The block's hash is computed over a stable JSON serialization of a subset of fields from the `Block` structure:
1. **`index`** (int): Positional index of the block.
2. **`timestamp`** (int64): Unix timestamp when the block was mined.
3. **`transactions`** ([]Transaction): List of transactions packed in the block.
4. **`previousHash`** (string): Cryptographic link to the preceding block.
5. **`nonce`** (int): Counter used in Proof-of-Work mining.

#### Rationale for Excluding the Hash Field
The block's own `Hash` field is explicitly excluded from the serialization. If it were included, calculating the hash would change the block's data, which in turn would invalidate the computed hash—creating an infinite recursion problem. 

To achieve this cleanly, a dedicated serialization structure `blockHashInput` is defined inside `utils`:
```go
type blockHashInput struct {
	Index        int                       `json:"index"`
	Timestamp    int64                     `json:"timestamp"`
	Transactions []transaction.Transaction `json:"transactions"`
	PreviousHash string                    `json:"previousHash"`
	Nonce        int                       `json:"nonce"`
}
```
Because Go struct fields are serialized by `encoding/json` in the exact order they are declared, this representation is 100% stable and compiler-independent.

### 1.2 Full-Chain Validation and Integrity Guarantees
The chain validation routine in [blockchain/blockchain.go](file:///c:/intern/toy-blockchain/blockchain/blockchain.go) guarantees integrity through a series of sequential checks:

```mermaid
graph TD
    A[Start Validate] --> B{Verify Genesis Block}
    B -- Fails --o C[Report Failure Index 0]
    B -- Passes --> D[Loop i from 1 to N-1]
    D --> E{Index Sequential? i == Blocks[i].Index}
    E -- No --o F[Report Failure Index i]
    E -- Yes --> G{PrevHash Link Matches? Blocks[i].PreviousHash == Blocks[i-1].Hash}
    G -- No --o F
    G -- Yes --> H{Recalculate Hash Match? Blocks[i].Hash == Recalculate}
    H -- No --o F
    H -- Yes --> I{PoW Target Met? Hash prefix has difficulty zeros}
    I -- No --o F
    I -- Yes --> J{Timestamp Chronological? Blocks[i].Timestamp >= Blocks[i-1].Timestamp}
    J -- No --o F
    J -- Yes --> K[Loop next block]
    D --> L[Validation Passed: Return true]
```

This sequence guarantees that any modification to transactions, timestamps, nonces, or previous hashes anywhere in the chain breaks the cryptographic links, making the validation fail immediately at the earliest tampered block.

---

## 2. Tamper-Evidence Experiment

### 2.1 Methodology
An honest blockchain was initialized and populated with transactions, then mined up to block height 2.
- **Genesis Block (Index 0)**: Empty transactions, `PreviousHash = "0"`.
- **Block 1**: 1 transaction: `faucet` sends `100.0` to `alice`.
- **Block 2**: 1 transaction: `alice` sends `40.0` to `bob`.

Validation was executed successfully on this honest state. To test tamper detection, the transaction amount inside **Block 1** was manually altered from `100.0` to `150.0` in the database file `blockchain.json`.

### 2.2 CLI Outputs Before and After Modification

#### Before Modification (Honest State)
```bash
$ ./toy-blockchain -difficulty 3 validate
Blockchain is VALID (integrity check passed).
```

#### After Modifying Block 1's Transaction Amount (from 100 to 150)
```bash
$ ./toy-blockchain -difficulty 3 validate
Blockchain is INVALID!
  First offending block index: 1
  Reason: block hash mismatch: block 1 stores hash '0008de9a7832f67e4920b2b9757f44ab0f61b138ff30a24a8d308f0b7a4f3df6', but recalculated hash is '35c479adab184ee0d603a1147a2ad8be027e1f4bde4a3875323a650d55e81d77'
```

### 2.3 Analysis of Failure Catching
* **Recalculation Check (First Line of Defense)**: The alteration of the transaction amount inside Block 1 changes the serialized JSON content of that block. Consequently, when `utils.CalculateHash(Blocks[1])` is executed, it yields a completely different SHA-256 hash (`35c479...`) compared to the mined hash stored in the block (`0008de...`).
* **Cascade Effect**: If the attacker recalculates the hash of Block 1 to match the new content, Block 2's `previousHash` field (which stores `0008de...`) will no longer match Block 1's new hash. If the attacker updates Block 2's `previousHash`, Block 2's own hash changes, which invalidates Block 3, and so on. This cascade makes tampering easily detectable.

---

## 3. Difficulty versus Effort Experiment

### 3.1 Empirical Results
The mining process was benchmarked across difficulty levels 1 through 6 on a single CPU core. A difficulty level of $N$ requires the hexadecimal representation of the block hash to start with $N$ leading zeros.

| Difficulty | Hashes Tried (Nonce) | Time Taken | Average Hash Rate (H/s) |
| :---: | :---: | :---: | :---: |
| **1** | 28 | 316.7µs | 88,411.75 |
| **2** | 207 | 1.567ms | 132,099.55 |
| **3** | 272 | 2.4412ms | 111,420.61 |
| **4** | 12,412 | 32.05ms | 387,259.02 |
| **5** | 12,412 | 10.95ms | 1,133,702.34 |
| **6** | 17,991,586 | 25.01s | 719,367.98 |

### 3.2 Trend Analysis and Mathematical Modeling
The SHA-256 output is pseudorandom and uniformly distributed. In hexadecimal representation, each character represents 4 bits and has 16 possible values (`0-9`, `a-f`). The probability that a random hash has a leading zero in a specific position is $\frac{1}{16}$.

Thus, for a difficulty $d$, the probability $P$ of finding a valid hash on any single attempt is:
$$P(d) = \left(\frac{1}{16}\right)^d = 16^{-d}$$

The expected number of attempts (hashes tried) before finding a valid block is a geometric distribution with expectation:
$$E[\text{Hashes}] = 16^d$$

#### Theoretical vs. Empirical Growth
* **Exponential scaling**: The workload grows exponentially by a factor of 16 for each difficulty increment, i.e., $O(16^d)$.
* **Anomalous Overlap at Difficulty 4 and 5**: 
  Interestingly, in our test, both difficulty 4 and difficulty 5 completed on the exact same nonce (`12,412`). This is a standard random variable variance event: the block hash discovered at nonce `12,412` happened to start with *five* leading zeros (e.g. `00000a...`). Because five leading zeros is a subset of four leading zeros, it satisfied both difficulty requirements simultaneously.
* **Difficulty 6 Spike**: As expected, difficulty 6 required **17,991,586** hashes (very close to the theoretical average of $16^6 = 16,777,216$), taking **25.01 seconds** to compute.

---

## 4. Discussion Questions

### 4.1 Tamper-Resistance: Toy vs. Production Chains
In a local "toy" blockchain, tampering is trivial. Since the state is stored in a single JSON file (`blockchain.json`), an attacker can easily modify a block, loop through all subsequent blocks to recalculate their hashes and link them back together, and rewrite the file. This takes milliseconds.

In a production, distributed blockchain, this is prevented by **decentralized consensus and massive computational work**:
1. **Network Consensus (Longest Chain Rule)**: Even if an attacker tampers with their local database, other nodes will reject it. The network only accepts the valid chain that represents the most cumulative proof-of-work (the heaviest chain).
2. **Economic and Computational Cost (51% Attack)**: To force the rest of the network to accept a tampered history, the attacker must mine block modifications faster than the rest of the network combined. This requires controlling >50% of the network's total hashing power, costing billions of dollars in hardware and electricity (e.g., in Bitcoin), making tampering economically irrational.

### 4.2 Proof-of-Work Alternatives

#### Proof-of-Stake (PoS)
Instead of allocating block-creation rights based on computational power, PoS assigns block creators (validators) proportionally to the number of native coins they lock up (stake) as collateral.
* **Advantage**: Extreme energy efficiency. It eliminates the massive electricity consumption of PoW mining rigs, allowing high-throughput transaction speeds on standard servers.
* **Drawback**: "Rich get richer" centralization pressure. Those who hold the most coins earn the highest staking rewards, compounding their wealth and power over governance. It also introduces complex security concerns like the *Nothing-at-Stake* problem.

### 4.3 Key Differences: Toy vs. Production Blockchains
1. **Consensus Protocols**: The toy blockchain runs in a single process and trusts the single database file. Production chains run peer-to-peer (P2P) nodes using gossip protocols and consensus algorithms (e.g., Nakamoto Consensus, PBFT, Casper) to agree on the state of the chain.
2. **Transaction Integrity via Signatures**: In our toy system, anyone can submit a transaction claiming to spend money from `alice`'s account since transactions are plain text. A production blockchain requires transactions to be cryptographically signed by the sender's private key and verified by the network using the sender's public key.
3. **Transaction Packaging (Merkle Trees)**: The toy blockchain stores transactions as a linear slice, meaning validation must process all transactions in a block. Production chains organize transactions in a Merkle Tree (a cryptographic binary tree). The block header only contains the Merkle Root hash, enabling Light Clients (SPV) to verify that a transaction is included in a block without downloading the entire chain data.

#### Design Sketch: Adding Cryptographic Transaction Signatures
To secure transactions in this toy blockchain:
1. **Key Pairs**: Each user generates an ECDSA key pair (using Go's standard library `crypto/ecdsa` and `crypto/elliptic` on the P-256 curve). The public key serves as their account address (or a hash of it), and the private key is kept secret.
2. **Transaction Fields**: Add a `Signature` field to the `Transaction` struct:
   ```go
   type Transaction struct {
       Sender    string `json:"sender"`    // User's serialized Public Key
       Receiver  string `json:"receiver"`  // Receiver's serialized Public Key
       Amount    float64 `json:"amount"`
       Signature []byte  `json:"signature"`
   }
   ```
3. **Signing**: Before sending, the client hashes the transaction data (excluding the signature) and signs it using the private key:
   ```go
   func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) error {
       hash := tx.CalculateDataHash() // Hash of sender, receiver, amount
       r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash)
       if err != nil { return err }
       tx.Signature = append(r.Bytes(), s.Bytes()...)
       return nil
   }
   ```
4. **Verification**: During transaction addition and block validation, the nodes verify the signature using the sender's public key:
   ```go
   func (bc *Blockchain) VerifyTransaction(tx Transaction) error {
       // ... existing balance checks ...
       publicKey := DecodePublicKey(tx.Sender)
       hash := tx.CalculateDataHash()
       if !ecdsa.Verify(publicKey, hash, tx.Signature) {
           return errors.New("invalid signature: sender identity check failed")
       }
       return nil
   }
   ```
This cryptographic defense guarantees that only the owner of the private key can spend funds from an account.
