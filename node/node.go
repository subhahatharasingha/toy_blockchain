package node

import (
	"toy-blockchain/blockchain"
)

// Node represents a single blockchain node in the network.
type Node struct {
	ID         string
	Host       string
	Port       int
	Blockchain *blockchain.Blockchain
}

// NewNode creates and initializes a new Node.
// If bc is nil, a default blockchain instance is initialized.
func NewNode(id string, host string, port int, bc *blockchain.Blockchain) *Node {
	if bc == nil {
		bc = blockchain.NewBlockchain()
	}
	return &Node{
		ID:         id,
		Host:       host,
		Port:       port,
		Blockchain: bc,
	}
}
