package node_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"toy-blockchain/block"
	"toy-blockchain/blockchain"
	toy_mining "toy-blockchain/mining"
	"toy-blockchain/node"
	"toy-blockchain/transaction"
	"toy-blockchain/utils"
	"toy-blockchain/wallet"
)

func TestNewNode(t *testing.T) {
	id := "test-node-1"
	host := "127.0.0.1"
	port := 8000

	n1 := node.NewNode(id, host, port, nil)
	if n1 == nil {
		t.Fatal("Expected NewNode to return a non-nil pointer")
	}

	if n1.ID != id {
		t.Errorf("Expected ID %s, got %s", id, n1.ID)
	}

	if n1.Host != host {
		t.Errorf("Expected Host %s, got %s", host, n1.Host)
	}

	if n1.Port != port {
		t.Errorf("Expected Port %d, got %d", port, n1.Port)
	}

	if n1.Blockchain == nil {
		t.Fatal("Expected Blockchain to be non-nil")
	}
}

func TestNewNodeWithCustomBlockchain(t *testing.T) {
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
	n1 := node.NewNode("node-a", "127.0.0.1", 8000, nil)
	n2 := node.NewNode("node-b", "127.0.0.1", 8001, nil)

	if n1.Blockchain == n2.Blockchain {
		t.Error("Independent nodes should not share the same Blockchain instance")
	}
}

func TestNodeConfigurationInspectableWithoutServer(t *testing.T) {
	n := node.NewNode("node-c", "192.168.1.100", 9000, nil)
	if n.ID != "node-c" || n.Host != "192.168.1.100" || n.Port != 9000 {
		t.Error("Failed to inspect node configuration correctly")
	}
}

func TestNodeHTTPServerLifecycleAndEndpoints(t *testing.T) {
	n := node.NewNode("test-node-lifecycle", "127.0.0.1", 0, nil)

	if n.IsServerRunning() {
		t.Fatal("Expected server not to be running initially")
	}

	err := n.StartServer()
	if err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer func() {
		_ = n.Shutdown(context.Background())
	}()

	if !n.IsServerRunning() {
		t.Fatal("Expected IsServerRunning() to return true")
	}

	if n.Port == 0 {
		t.Fatal("Expected Port to be updated from 0 to the dynamically allocated port")
	}

	client := &http.Client{Timeout: 1 * time.Second}
	urlHealth := fmt.Sprintf("http://%s:%d/health", n.Host, n.Port)
	urlRoot := fmt.Sprintf("http://%s:%d/", n.Host, n.Port)

	for _, url := range []string{urlHealth, urlRoot} {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Failed to send request to %s: %v", url, err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d for URL: %s", resp.StatusCode, url)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var healthResp struct {
			Status string `json:"status"`
		}
		err = json.Unmarshal(bodyBytes, &healthResp)
		if err != nil {
			t.Fatalf("Failed to parse json body: %v", err)
		}

		if healthResp.Status != "OK" {
			t.Errorf("Expected status 'OK', got '%s'", healthResp.Status)
		}
	}

	err = n.StartServer()
	if err == nil {
		t.Error("Expected error when starting an already running server, got nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = n.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shut down server cleanly: %v", err)
	}

	if n.IsServerRunning() {
		t.Fatal("Expected IsServerRunning() to return false after shutdown")
	}
}

func TestMultipleHTTPServerNodes(t *testing.T) {
	n1 := node.NewNode("node-1", "127.0.0.1", 0, nil)
	n2 := node.NewNode("node-2", "127.0.0.1", 0, nil)

	err := n1.StartServer()
	if err != nil {
		t.Fatalf("Failed to start node 1 server: %v", err)
	}
	defer func() { _ = n1.Shutdown(context.Background()) }()

	err = n2.StartServer()
	if err != nil {
		t.Fatalf("Failed to start node 2 server: %v", err)
	}
	defer func() { _ = n2.Shutdown(context.Background()) }()

	if n1.Port == n2.Port {
		t.Errorf("Expected nodes to allocate different dynamic ports, but both got %d", n1.Port)
	}
}

func TestNodeEmptyPeersInitially(t *testing.T) {
	n := node.NewNode("test-node", "127.0.0.1", 8000, nil)
	peers := n.GetPeers()
	if len(peers) != 0 {
		t.Errorf("Expected peer list to be empty initially, got %v", peers)
	}
}

func TestAddAndGetPeer(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}

	err := n.AddPeer(p)
	if err != nil {
		t.Fatalf("Expected AddPeer to succeed, got error: %v", err)
	}

	peers := n.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("Expected exactly 1 peer, got %d", len(peers))
	}
}

func TestNodeCannotAddSelfAsPeer(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)

	pSelfID := node.Peer{ID: "node-self", Host: "127.0.0.2", Port: 8001}
	err := n.AddPeer(pSelfID)
	if err == nil {
		t.Error("Expected error when adding self as peer (matching ID), got nil")
	}
}

func TestRemovePeer(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}
	_ = n.AddPeer(p)

	err := n.RemovePeer("peer-1")
	if err != nil {
		t.Fatalf("Expected RemovePeer to succeed, got: %v", err)
	}

	if len(n.GetPeers()) != 0 {
		t.Error("Expected peers list to be empty after peer removal")
	}
}

func TestNodeIntrospectionAPI(t *testing.T) {
	n := node.NewNode("introspection-node", "127.0.0.1", 0, nil)

	err := n.StartServer()
	if err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer func() {
		_ = n.Shutdown(context.Background())
	}()

	client := &http.Client{Timeout: 1 * time.Second}
	urlNode := fmt.Sprintf("http://%s:%d/node", n.Host, n.Port)
	urlPeers := fmt.Sprintf("http://%s:%d/peers", n.Host, n.Port)

	respNode, err := client.Get(urlNode)
	if err != nil {
		t.Fatalf("Failed GET /node: %v", err)
	}
	_ = respNode.Body.Close()
	if respNode.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for GET /node, got %d", respNode.StatusCode)
	}

	respPeers, err := client.Get(urlPeers)
	if err != nil {
		t.Fatalf("Failed GET /peers: %v", err)
	}
	_ = respPeers.Body.Close()
	if respPeers.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for GET /peers, got %d", respPeers.StatusCode)
	}
}

func TestTransactionDuplicatePrevention(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: aliceWallet.Address, Amount: 100.0}
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)

	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeB)

	tx, err := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}

	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(tx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post to node A failed: %v", err)
	}
	_ = resp.Body.Close()

	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Node B did not receive gossiped transaction: %v", err)
	}

	resp2, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeB.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post duplicate to node B failed: %v", err)
	}
	_ = resp2.Body.Close()

	time.Sleep(100 * time.Millisecond)
	if len(nodeB.GetPendingTransactions()) != 1 {
		t.Errorf("Node B pending transactions pool increased on duplicate transaction; expected 1, got %d", len(nodeB.GetPendingTransactions()))
	}
}

// Phase 4 Tests

func TestGetChainEndpoint(t *testing.T) {
	n := node.NewNode("node-chain-endpoint", "127.0.0.1", 0, nil)
	_ = n.StartServer()
	defer func() { _ = n.Shutdown(context.Background()) }()

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/chain", n.Port))
	if err != nil {
		t.Fatalf("GET /chain failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var blocks []block.Block
	err = json.NewDecoder(resp.Body).Decode(&blocks)
	if err != nil {
		t.Fatalf("Failed to decode chain response: %v", err)
	}

	if len(blocks) != 1 {
		t.Errorf("Expected genesis block to be returned, got %d blocks", len(blocks))
	}
}

func TestChainValidation(t *testing.T) {
	bc := blockchain.NewBlockchain()
	if !blockchain.ValidateChain(bc.Blocks) {
		t.Error("Standard initialized chain should validate successfully")
	}

	invalidChain := []block.Block{
		bc.Blocks[0],
		{Index: 5, PreviousHash: bc.Blocks[0].Hash},
	}
	if blockchain.ValidateChain(invalidChain) {
		t.Error("Chain with index out of sequence should be invalid")
	}
}

func TestInvalidPeerChainRejected(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	corruptBlock := block.Block{
		Index:        1,
		PreviousHash: "corrupt-prev-hash",
		Hash:         "invalid-pow-hash",
	}
	nodeB.AddMinedBlock(corruptBlock)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err == nil {
		t.Error("Expected synchronization with invalid peer chain to fail")
	}

	if len(nodeA.GetBlocks()) != 1 {
		t.Errorf("Node A local state modified by invalid chain; expected 1 block, got %d", len(nodeA.GetBlocks()))
	}
}

func TestShorterChainRejected(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	mineBlockOnNode(nodeA)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err == nil {
		t.Error("Expected sync with shorter chain to fail")
	}

	if len(nodeA.GetBlocks()) != 2 {
		t.Errorf("Node A local chain replaced by shorter chain; expected 2 blocks, got %d", len(nodeA.GetBlocks()))
	}
}

func TestPreferredChainAccepted(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Synchronization failed: %v", err)
	}

	if len(nodeA.GetBlocks()) != 3 {
		t.Errorf("Expected Node A to adopt Node B's preferred chain; expected 3 blocks, got %d", len(nodeA.GetBlocks()))
	}
}

func TestFindCommonAncestor(t *testing.T) {
	bc := blockchain.NewBlockchain()
	b1 := mineNextTestBlock(bc.Blocks[0])
	b2a := mineNextTestBlock(b1)
	
	// Diverge by setting a different timestamp for b2b
	b2b := b1
	b2b.Timestamp += 5
	b2b = mineNextTestBlock(b2b)

	chainA := []block.Block{bc.Blocks[0], b1, b2a}
	chainB := []block.Block{bc.Blocks[0], b1, b2b}

	idx, err := blockchain.FindCommonAncestor(chainA, chainB)
	if err != nil {
		t.Fatalf("FindCommonAncestor failed: %v", err)
	}
	if idx != 1 {
		t.Errorf("Expected common ancestor at index 1, got %d", idx)
	}
}

func TestForkDetection(t *testing.T) {
	bc := blockchain.NewBlockchain()
	b1 := mineNextTestBlock(bc.Blocks[0])
	b2a := mineNextTestBlock(b1)
	
	b2b := b1
	b2b.Timestamp += 5
	b2b = mineNextTestBlock(b2b)

	chainA := []block.Block{bc.Blocks[0], b1, b2a}
	chainB := []block.Block{bc.Blocks[0], b1, b2b}

	ancestorIdx, err := blockchain.FindCommonAncestor(chainA, chainB)
	if err != nil {
		t.Fatalf("Failed to detect fork common ancestor: %v", err)
	}

	if ancestorIdx != 1 {
		t.Errorf("Expected divergence after index 1, got ancestor index %d", ancestorIdx)
	}
}

func TestChainReorganization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	mineBlockOnNode(nodeA)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	mineBlockOnNode(nodeA)

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Reorganization failed: %v", err)
	}

	if len(nodeA.GetBlocks()) != 4 {
		t.Fatalf("Expected 4 blocks after reorganization, got %d", len(nodeA.GetBlocks()))
	}

	if nodeA.GetBlocks()[3].Hash != nodeB.GetBlocks()[3].Hash {
		t.Errorf("Tip hash mismatch after reorganization")
	}
}

func TestOrphanedTransactionRestoration(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)
	mineBlockOnNode(nodeA)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	t1, _ := transaction.NewSignedTransaction(alice, bob.Address, 10.0)
	_ = nodeA.AddTransaction(t1)

	mineBlockOnNode(nodeA)

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Synchronization failed: %v", err)
	}

	pending := nodeA.GetPendingTransactions()
	if len(pending) != 1 || pending[0].ID != t1.ID {
		t.Errorf("Orphaned transaction T1 was not restored to pending pool, pending: %v", pending)
	}
}

func TestDuplicateTransactionNotRestored(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)
	mineBlockOnNode(nodeA)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	t2, _ := transaction.NewSignedTransaction(alice, bob.Address, 10.0)
	
	_ = nodeA.AddTransaction(t2)
	mineBlockOnNode(nodeA)

	_ = nodeB.AddTransaction(t2)
	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(nodeA.GetPendingTransactions()) != 0 {
		t.Errorf("Duplicate transaction restored to mempool: %v", nodeA.GetPendingTransactions())
	}
}

func TestMempoolConsistencyAfterReorganization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)
	mineBlockOnNode(nodeA)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	t1, _ := transaction.NewSignedTransaction(alice, bob.Address, 80.0)
	t2, _ := transaction.NewSignedTransaction(alice, bob.Address, 50.0)

	_ = nodeA.AddTransaction(t1)
	mineBlockOnNode(nodeA)

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	_ = nodeA.AddTransaction(t2)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	pending := nodeA.GetPendingTransactions()
	if len(pending) != 1 || pending[0].ID != t1.ID {
		t.Errorf("Mempool became inconsistent or restoration ordering was violated: %v", pending)
	}
}

func TestBalanceAfterReorganization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()

	faucetTxA := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	faucetTxB := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 250.0}

	_ = nodeA.AddTransaction(faucetTxA)
	mineBlockOnNode(nodeA)

	_ = nodeB.AddTransaction(faucetTxB)
	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	balance := nodeA.Blockchain.GetBalance(alice.Address)
	if balance != 250.0 {
		t.Errorf("Balance not updated to reflect adopted chain: expected 250.0, got %f", balance)
	}
}

func TestOutOfOrderBlockHandling(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeB)

	// Now connect as peers
	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(nodeB.GetBlocks()[2])
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/blocks", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t.Errorf("Expected out-of-order block to be rejected by POST /blocks, got %d", resp.StatusCode)
	}

	err = waitForCondition(func() bool {
		return len(nodeA.GetBlocks()) == 3
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("Node A did not automatically synchronize to fill out-of-order gaps: %v", err)
	}
}

func TestPeerSynchronizationFailure(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	_ = nodeA.StartServer()
	defer func() { _ = nodeA.Shutdown(context.Background()) }()

	offlinePeer := node.Peer{ID: "node-offline", Host: "127.0.0.1", Port: 54321}
	err := nodeA.SyncWithPeer(offlinePeer)
	if err == nil {
		t.Error("Expected synchronization with offline peer to fail")
	}

	if !nodeA.IsServerRunning() {
		t.Error("Node A server stopped running or crashed due to offline peer")
	}
}

func TestConcurrentSynchronization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)
	nodeC := node.NewNode("node-c", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	_ = nodeC.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
		_ = nodeC.Shutdown(context.Background())
	}()

	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeC)

	done := make(chan bool)
	go func() {
		_ = nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
		done <- true
	}()
	go func() {
		_ = nodeA.SyncWithPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
		done <- true
	}()

	<-done
	<-done
}

func TestMiningAfterReorganization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	_ = nodeB.AddTransaction(faucetTx)
	mineBlockOnNode(nodeB)

	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	tx, _ := transaction.NewSignedTransaction(alice, alice.Address, 1.0)
	_ = nodeA.AddTransaction(tx)
	mineBlockOnNode(nodeA)

	blocks := nodeA.GetBlocks()
	if len(blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(blocks))
	}

	if blocks[2].PreviousHash != blocks[1].Hash {
		t.Errorf("Mined block previous hash pointer is incorrect after synchronization")
	}
}

func TestThreeNodeChainSynchronization(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)
	nodeC := node.NewNode("node-c", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	_ = nodeC.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
		_ = nodeC.Shutdown(context.Background())
	}()

	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
	_ = nodeC.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	mineBlockOnNode(nodeA)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})
	_ = nodeC.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	mineBlockOnNode(nodeA)

	mineBlockOnNode(nodeC)
	mineBlockOnNode(nodeC)

	// Converge the network synchronously:
	// Node B pulls from C (getting length 4)
	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
	// Node A pulls from B (getting length 4)
	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	err := waitForCondition(func() bool {
		return len(nodeA.GetBlocks()) == 4 && len(nodeB.GetBlocks()) == 4 && len(nodeC.GetBlocks()) == 4
	}, 5*time.Second)

	if err != nil {
		t.Fatalf("Nodes failed to converge: A has %d, B has %d, C has %d blocks",
			len(nodeA.GetBlocks()), len(nodeB.GetBlocks()), len(nodeC.GetBlocks()))
	}
}

func TestExistingTransactionGossipStillWorks(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	alice, _ := wallet.NewWallet()
	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 10.0}
	faucetTx.ID, _ = faucetTx.CalculateID()
	
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(faucetTx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Gossip post failed: %v", err)
	}
	_ = resp.Body.Close()

	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Errorf("Transaction gossip is broken: %v", err)
	}
}

func TestExistingBlockGossipStillWorks(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	mineBlockOnNode(nodeA)

	err := waitForCondition(func() bool {
		return len(nodeB.GetBlocks()) == 2
	}, 3*time.Second)
	if err != nil {
		t.Errorf("Block gossip is broken: %v", err)
	}
}

func TestInvalidChainDoesNotModifyLocalState(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	alice, _ := wallet.NewWallet()
	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: alice.Address, Amount: 100.0}
	_ = nodeA.AddTransaction(faucetTx)
	mineBlockOnNode(nodeA)

	corruptBlock := nodeA.GetBlocks()[1]
	corruptBlock.MerkleRoot = "corrupted-merkle-root"
	nodeB.AddMinedBlock(corruptBlock)

	err := nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if err == nil {
		t.Error("Expected sync with corrupted chain to fail")
	}

	if len(nodeA.GetBlocks()) != 2 || nodeA.GetBlocks()[1].MerkleRoot == "corrupted-merkle-root" {
		t.Error("Local blockchain state was corrupted by invalid synchronization attempt")
	}
}

func TestCumulativeDifficultyChainSelection(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	// 1. Higher cumulative difficulty beats a longer but lower-work chain.
	// Mine Node A (higher work, shorter length: length 7, work 9)
	for i := 1; i <= 6; i++ {
		blocks := nodeA.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+1)
		nodeA.AddMinedBlock(b)
	}

	// Mine Node B (lower work, longer length: length 8, work 7)
	for i := 1; i <= 7; i++ {
		blocks := nodeB.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+300)
		nodeB.AddMinedBlock(b)
	}

	// Sync Node A with Node B. A has work 9, B has work 7. A must keep its chain.
	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if len(nodeA.GetBlocks()) != 7 {
		t.Errorf("Node A adopted lower work chain; expected 7 blocks, got %d", len(nodeA.GetBlocks()))
	}

	// Sync Node B with Node A. B has work 7, A has work 9. B must adopt A's chain!
	err := nodeB.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(nodeB.GetBlocks()) != 7 {
		t.Errorf("Node B did not adopt higher work chain; expected 7 blocks, got %d", len(nodeB.GetBlocks()))
	}

	// 2. Equal cumulative difficulty prefers the longer chain.
	nodeC := node.NewNode("node-c", "127.0.0.1", 0, nil)
	_ = nodeC.StartServer()
	defer func() { _ = nodeC.Shutdown(context.Background()) }()

	// Node A mines 1 block with 1s increment (total work = 9 + 4 = 13, length = 8)
	blocksA := nodeA.GetBlocks()
	bA8 := mineNextChainBlock(blocksA, blocksA[len(blocksA)-1].Timestamp+1)
	nodeA.AddMinedBlock(bA8)

	// Mine Node C from Genesis with 300s spacing to reach work 13 (requires 13 blocks)
	for i := 1; i <= 13; i++ {
		blocks := nodeC.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+300)
		nodeC.AddMinedBlock(b)
	}

	workA := blockchain.CalculateCumulativeDifficulty(nodeA.GetBlocks())
	workC := blockchain.CalculateCumulativeDifficulty(nodeC.GetBlocks())
	fmt.Printf("DEBUG TEST: A work=%d len=%d, C work=%d len=%d\n", workA, len(nodeA.GetBlocks()), workC, len(nodeC.GetBlocks()))

	// Sync Node A with Node C.
	// Equal work (13 == 13), but Node C is longer (14 > 8). Node A must adopt C's chain.
	err = nodeA.SyncWithPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(nodeA.GetBlocks()) != 14 {
		t.Errorf("Node A did not adopt longer chain on equal work; expected 14 blocks, got %d", len(nodeA.GetBlocks()))
	}

	// 3. Equal work and equal length keeps the existing chain.
	nodeD := node.NewNode("node-d", "127.0.0.1", 0, nil)
	_ = nodeD.StartServer()
	defer func() { _ = nodeD.Shutdown(context.Background()) }()
	_ = nodeD.SyncWithPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})

	// Node A mines block 11 (work = 13 + 1 = 14, length = 12)
	blocksA = nodeA.GetBlocks()
	bA10 := mineNextChainBlock(blocksA, blocksA[len(blocksA)-1].Timestamp+300)
	nodeA.AddMinedBlock(bA10)

	// Node D mines block 11 with different content (work = 13 + 1 = 14, length = 12)
	blocksD := nodeD.GetBlocks()
	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: "alice", Amount: 5.0}
	faucetTx.ID, _ = faucetTx.CalculateID()
	_ = nodeD.AddTransaction(faucetTx)
	bD10 := mineNextChainBlock(blocksD, blocksD[len(blocksD)-1].Timestamp+300)
	nodeD.AddMinedBlock(bD10)

	originalHash := nodeA.GetBlocks()[11].Hash
	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeD.ID, Host: nodeD.Host, Port: nodeD.Port})

	if nodeA.GetBlocks()[11].Hash != originalHash {
		t.Error("Node A replaced its chain when peer had equal work and equal length")
	}
}

// Helpers
var dummyCounter int64

func mineBlockOnNode(n *node.Node) {
	txs := n.GetPendingTransactions()
	if len(txs) == 0 {
		dummy := transaction.Transaction{
			Sender:    "faucet",
			Receiver:  "dummy",
			Amount:    1.0,
			Timestamp: time.Now().UnixNano() + atomic.AddInt64(&dummyCounter, 1),
		}
		dummy.ID, _ = dummy.CalculateID()
		err := n.AddTransaction(dummy)
		if err != nil {
			panic(fmt.Sprintf("Failed to add dummy transaction: %v", err))
		}
	}

	blockData, err := n.CreatePendingBlock(10)
	if err != nil {
		panic(err)
	}
	blocks := n.GetBlocks()
	blockData.Timestamp = blocks[len(blocks)-1].Timestamp + 1
	toy_mining.MineBlock(&blockData, blockData.Difficulty)
	n.AddMinedBlockAndGossip(blockData)
}


func mineNextTestBlock(prev block.Block) block.Block {
	b := block.Block{
		Index:        prev.Index + 1,
		Timestamp:    prev.Timestamp + 1,
		PreviousHash: prev.Hash,
		Difficulty:   prev.Difficulty,
		MerkleRoot:   utils.CalculateMerkleRoot(nil),
	}
	if b.Difficulty < 1 {
		b.Difficulty = 1
	}
	toy_mining.MineBlock(&b, b.Difficulty)
	return b
}

func mineNextChainBlock(chain []block.Block, ts int64) block.Block {
	prev := chain[len(chain)-1]
	diff := blockchain.CalculateNextDifficulty(chain)
	b := block.Block{
		Index:        prev.Index + 1,
		Timestamp:    ts,
		PreviousHash: prev.Hash,
		Difficulty:   diff,
		MerkleRoot:   utils.CalculateMerkleRoot(nil),
	}
	toy_mining.MineBlock(&b, diff)
	return b
}


func waitForCondition(cond func() bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("condition not met within timeout")
}
