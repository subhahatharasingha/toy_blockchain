package block

import "toy-blockchain/transaction"

// Block represents one block in the blockchain.
type Block struct {
	Index        int                         `json:"index"`
	Timestamp    int64                       `json:"timestamp"`
	Transactions []transaction.Transaction   `json:"transactions"`
	PreviousHash string                      `json:"previousHash"`
	Nonce        int                         `json:"nonce"`
	Difficulty   int                         `json:"difficulty"`
	Hash         string                      `json:"hash"`
}