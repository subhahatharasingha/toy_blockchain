package node_test

import (
	"testing"
	"toy-blockchain/blockchain"
	"toy-blockchain/node"
)

func TestNewNode(t *testing.T) {
	id := "test-node-1"
	host := "127.0.0.1"
	port := 8000

	// 1. Success case with nil blockchain (defaults to a new one)
	n1 := node.NewNode(id, host, port, nil)
	if n1 == nil {
		t.Fatal("Expected NewNode to return a non-nil pointer")
	}

	// 2. Node has expected ID
	if n1.ID != id {
		t.Errorf("Expected ID %s, got %s", id, n1.ID)
	}

	// 3. Node has expected host/address
	if n1.Host != host {
		t.Errorf("Expected Host %s, got %s", host, n1.Host)
	}

	// 4. Node has expected port
	if n1.Port != port {
		t.Errorf("Expected Port %d, got %d", port, n1.Port)
	}

	// 5. Node has a non-nil Blockchain
	if n1.Blockchain == nil {
		t.Fatal("Expected Blockchain to be non-nil")
	}
}

func TestNewNodeWithCustomBlockchain(t *testing.T) {
	// Verify that when NewNode() receives a non-nil Blockchain,
	// the Node references the exact same Blockchain instance and does not create a replacement.
	customBC := blockchain.NewBlockchain()
	n := node.NewNode("test-node-custom", "localhost", 8001, customBC)

	if n.Blockchain == nil {
		t.Fatal("Expected Node blockchain to be non-nil")
	}

	if n.Blockchain != customBC {
		t.Error("Expected Node to reference the exact same supplied Blockchain instance")
	}
}

func TestIndependentNodesDoNotShareBlockchain(t *testing.T) {
	// 6. Two independently created nodes do not accidentally share the same Blockchain instance
	n1 := node.NewNode("node-a", "127.0.0.1", 8000, nil)
	n2 := node.NewNode("node-b", "127.0.0.1", 8001, nil)

	if n1.Blockchain == n2.Blockchain {
		t.Error("Independent nodes should not share the same Blockchain instance")
	}
}

func TestNodeConfigurationInspectableWithoutServer(t *testing.T) {
	// 7. Node configuration can be inspected without starting a server
	n := node.NewNode("node-c", "192.168.1.100", 9000, nil)
	if n.ID != "node-c" || n.Host != "192.168.1.100" || n.Port != 9000 {
		t.Error("Failed to inspect node configuration correctly")
	}
}
