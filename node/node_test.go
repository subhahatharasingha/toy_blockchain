package node_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
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

func faucetTx(receiver string, amount float64) transaction.Transaction {
	tx, err := transaction.NewSignedSpecialTransaction("faucet", wallet.FaucetPrivateKey, wallet.FaucetPublicKey, receiver, amount)
	if err != nil {
		panic(err)
	}
	return tx
}

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

	faucetTx := faucetTx(aliceWallet.Address, 100.0)
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

	faucetTx := faucetTx(alice.Address, 100.0)
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

	faucetTx := faucetTx(alice.Address, 100.0)
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

	faucetTx := faucetTx(alice.Address, 100.0)
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

	faucetTxA := faucetTx(alice.Address, 100.0)
	faucetTxB := faucetTx(alice.Address, 250.0)

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

	// Connect peers afterwards
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

	faucetTx := faucetTx(alice.Address, 100.0)
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

	_ = nodeB.SyncWithPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
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
	faucetTx := faucetTx(alice.Address, 10.0)

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
	faucetTx := faucetTx(alice.Address, 100.0)
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
	for i := 1; i <= 6; i++ {
		blocks := nodeA.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+1)
		nodeA.AddMinedBlock(b)
	}

	for i := 1; i <= 7; i++ {
		blocks := nodeB.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+300)
		nodeB.AddMinedBlock(b)
	}

	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})
	if len(nodeA.GetBlocks()) != 7 {
		t.Errorf("Node A adopted lower work chain; expected 7 blocks, got %d", len(nodeA.GetBlocks()))
	}

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

	blocksA := nodeA.GetBlocks()
	bA8 := mineNextChainBlock(blocksA, blocksA[len(blocksA)-1].Timestamp+1)
	nodeA.AddMinedBlock(bA8)

	for i := 1; i <= 13; i++ {
		blocks := nodeC.GetBlocks()
		b := mineNextChainBlock(blocks, blocks[len(blocks)-1].Timestamp+300)
		nodeC.AddMinedBlock(b)
	}

	workA := blockchain.CalculateCumulativeDifficulty(nodeA.GetBlocks())
	workC := blockchain.CalculateCumulativeDifficulty(nodeC.GetBlocks())
	fmt.Printf("DEBUG TEST: A work=%d len=%d, C work=%d len=%d\n", workA, len(nodeA.GetBlocks()), workC, len(nodeC.GetBlocks()))

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

	blocksA = nodeA.GetBlocks()
	bA10 := mineNextChainBlock(blocksA, blocksA[len(blocksA)-1].Timestamp+300)
	nodeA.AddMinedBlock(bA10)

	blocksD := nodeD.GetBlocks()
	faucetTx := faucetTx("alice", 5.0)
	_ = nodeD.AddTransaction(faucetTx)
	bD10 := mineNextChainBlock(blocksD, blocksD[len(blocksD)-1].Timestamp+300)
	nodeD.AddMinedBlock(bD10)

	originalHash := nodeA.GetBlocks()[11].Hash
	_ = nodeA.SyncWithPeer(node.Peer{ID: nodeD.ID, Host: nodeD.Host, Port: nodeD.Port})

	if nodeA.GetBlocks()[11].Hash != originalHash {
		t.Error("Node A replaced its chain when peer had equal work and equal length")
	}
}

// Phase 5 Tests

func TestConcurrentStressNode(t *testing.T) {
	n := node.NewNode("stress-node", "127.0.0.1", 0, nil)
	err := n.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = n.Shutdown(context.Background()) }()

	alice, _ := wallet.NewWallet()
	faucetTx := faucetTx(alice.Address, 1000.0)
	_ = n.AddTransaction(faucetTx)
	mineBlockOnNode(n)

	var wgWorkers sync.WaitGroup
	var testErr error
	var errOnce sync.Once
	setErr := func(e error) {
		errOnce.Do(func() { testErr = e })
	}

	// Concurrently submit transactions, query introspection endpoints, add/remove peers, mine and synchronize
	client := &http.Client{Timeout: 1 * time.Second}
	workers := 20
	rounds := 30

	for w := 0; w < workers; w++ {
		wgWorkers.Add(1)
		go func(workerID int) {
			defer wgWorkers.Done()
			for r := 0; r < rounds; r++ {
				// 1. Submit transaction
				tx, err := transaction.NewSignedTransaction(alice, "bob", 0.01)
				if err == nil {
					tx.ID, _ = tx.CalculateID()
					data, _ := json.Marshal(tx)
					resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", n.Port), "application/json", bytes.NewBuffer(data))
					if err == nil {
						_ = resp.Body.Close()
					}
				}

				// 2. Query introspection
				resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/node", n.Port))
				if err == nil {
					_ = resp.Body.Close()
				}

				resp2, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/peers", n.Port))
				if err == nil {
					_ = resp2.Body.Close()
				}

				// 3. Add / Remove Peer
				p := node.Peer{
					ID:   fmt.Sprintf("peer-%d-%d", workerID, r),
					Host: "127.0.0.1",
					Port: 9000 + workerID,
				}
				_ = n.AddPeer(p)
				_ = n.RemovePeer(p.ID)

				// 4. Retrieve state copies
				_ = n.GetBlocks()
				_ = n.GetPendingTransactions()
				_ = n.GetPeers()
			}
		}(w)
	}

	// Mining worker
	var wgMining sync.WaitGroup
	stopMining := make(chan bool)
	wgMining.Add(1)
	go func() {
		defer wgMining.Done()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopMining:
				return
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							setErr(fmt.Errorf("Mining panicked: %v", r))
						}
					}()
					txs := n.GetPendingTransactions()
					if len(txs) > 0 {
						blockData, err := n.CreatePendingBlock(10)
						if err == nil {
							blocks := n.GetBlocks()
							blockData.Timestamp = blocks[len(blocks)-1].Timestamp + 300
							toy_mining.MineBlock(&blockData, blockData.Difficulty)
							n.AddMinedBlockAndGossip(blockData)
						}
					}
				}()
			}
		}
	}()

	wgWorkers.Wait()
	close(stopMining)
	wgMining.Wait()

	if testErr != nil {
		t.Fatalf("Stress test encountered error: %v", testErr)
	}
}

func TestThreeNodeClusterIntegration(t *testing.T) {
	// 1. Initialize three independent nodes on dynamic ports with independent blockchains
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, blockchain.NewBlockchain())
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, blockchain.NewBlockchain())
	nodeC := node.NewNode("node-c", "127.0.0.1", 0, blockchain.NewBlockchain())

	// Start all HTTP servers
	if err := nodeA.StartServer(); err != nil {
		t.Fatalf("Start Node A failed: %v", err)
	}
	if err := nodeB.StartServer(); err != nil {
		t.Fatalf("Start Node B failed: %v", err)
	}
	if err := nodeC.StartServer(); err != nil {
		t.Fatalf("Start Node C failed: %v", err)
	}

	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
		_ = nodeC.Shutdown(context.Background())
	}()

	// 2. Configure Ring Topology: A ↔ B, B ↔ C, C ↔ A
	peerA := node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port}
	peerB := node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port}
	peerC := node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port}

	_ = nodeA.AddPeer(peerB)
	_ = nodeA.AddPeer(peerC)
	_ = nodeB.AddPeer(peerA)
	_ = nodeB.AddPeer(peerC)
	_ = nodeC.AddPeer(peerA)
	_ = nodeC.AddPeer(peerB)

	// Seed faucet transaction to Node A, B, C to allow initial mining
	alice, _ := wallet.NewWallet()
	faucetTx := faucetTx(alice.Address, 1000.0)

	// Add faucet transaction to Node A and gossip it to B and C
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(faucetTx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post transaction failed: %v", err)
	}
	_ = resp.Body.Close()

	// 3-5. Verify transaction propagation to B and C
	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1 && len(nodeC.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("Transaction failed to propagate: Node B has %d, Node C has %d",
			len(nodeB.GetPendingTransactions()), len(nodeC.GetPendingTransactions()))
	}

	// 6. Ensure transaction exists only once in each mempool (no duplicates)
	if len(nodeA.GetPendingTransactions()) != 1 || len(nodeB.GetPendingTransactions()) != 1 {
		t.Errorf("Mempool size mismatch")
	}

	// 7-8. Mine a block containing faucetTx on Node A, and gossip it
	mineBlockOnNode(nodeA)

	// 9. Verify the block propagates to all nodes, clearing transaction from mempools
	err = waitForCondition(func() bool {
		return len(nodeA.GetBlocks()) == 2 && len(nodeB.GetBlocks()) == 2 && len(nodeC.GetBlocks()) == 2
	}, 4*time.Second)
	if err != nil {
		t.Fatalf("Block failed to propagate: Node A length=%d, Node B length=%d, Node C length=%d",
			len(nodeA.GetBlocks()), len(nodeB.GetBlocks()), len(nodeC.GetBlocks()))
	}

	// Mempools should be empty now
	if len(nodeA.GetPendingTransactions()) != 0 || len(nodeB.GetPendingTransactions()) != 0 {
		t.Errorf("Mempools were not cleared after block propagation")
	}

	// 10. Verify all nodes have the exact same tip chain
	if nodeA.GetBlocks()[1].Hash != nodeB.GetBlocks()[1].Hash || nodeA.GetBlocks()[1].Hash != nodeC.GetBlocks()[1].Hash {
		t.Errorf("Nodes converged to different blocks")
	}

	// 11. Create a Fork Scenario
	// Temporarily disconnect Node A and Node C from each other and from B to mine forks
	_ = nodeA.RemovePeer(nodeB.ID)
	_ = nodeA.RemovePeer(nodeC.ID)
	_ = nodeC.RemovePeer(nodeA.ID)
	_ = nodeC.RemovePeer(nodeB.ID)

	// Node A mines 2 blocks with 1s increments (higher work/difficulty adjustment)
	// B2_A: diff 1 (based on [Gen, B1] where B1 has diff 1)
	// B3_A: diff 2 (preceding has length 3, speed is high, adjustments triggered)
	// Node A total blocks = 4 (Gen, B1, B2_A, B3_A). Cumulative work = 1 (B1) + 1 (B2) + 2 (B3) = 4.
	// Node A mines 5 blocks with 1s increments (higher work/difficulty adjustment)
	for i := 1; i <= 5; i++ {
		blocksA := nodeA.GetBlocks()
		b := mineNextChainBlock(blocksA, blocksA[len(blocksA)-1].Timestamp+1)
		nodeA.AddMinedBlock(b)
	}
	// Node A total blocks = 7, cumulative work = 9.

	// Node C mines 3 blocks with 300s spacing (diff remains 1, lower work)
	for i := 1; i <= 3; i++ {
		blocksC := nodeC.GetBlocks()
		b := mineNextChainBlock(blocksC, blocksC[len(blocksC)-1].Timestamp+300)
		nodeC.AddMinedBlock(b)
	}
	// Node C total blocks = 5, cumulative work = 4.

	// Add a distinct signed transaction to Node C before it gets reorganized
	bob, _ := wallet.NewWallet()
	txC, _ := transaction.NewSignedTransaction(alice, bob.Address, 5.0)
	txC.ID, _ = txC.CalculateID()
	_ = nodeC.AddTransaction(txC)

	// 15. Verify offline peer: Node B is stopped
	err = nodeB.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown Node B: %v", err)
	}

	// 16. Re-enable Node B, reconnect peers, and trigger recovery synchronization
	nodeB.Port = 0 // Reset port to 0 to allocate a new dynamic port and avoid TIME_WAIT reuse issues
	if err := nodeB.StartServer(); err != nil {
		t.Fatalf("Failed to restart Node B: %v", err)
	}

	// Reconnect peers A ↔ B ↔ C ↔ A
	peerA.Port = nodeA.Port
	peerB.Port = nodeB.Port
	peerC.Port = nodeC.Port

	_ = nodeA.AddPeer(peerB)
	_ = nodeA.AddPeer(peerC)
	_ = nodeB.AddPeer(peerA)
	_ = nodeB.AddPeer(peerC)
	_ = nodeC.AddPeer(peerA)
	_ = nodeC.AddPeer(peerB)

	// Trigger synchronization explicitly and synchronously
	_ = nodeB.SyncWithPeer(peerA)
	_ = nodeC.SyncWithPeer(peerA)

	// 12-13. Verify preferred-chain selection (higher cumulative difficulty wins over length) and convergence
	err = waitForCondition(func() bool {
		return len(nodeA.GetBlocks()) == 7 && len(nodeB.GetBlocks()) == 7 && len(nodeC.GetBlocks()) == 7
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("Nodes failed to converge on preferred chain: A=%d, B=%d, C=%d blocks",
			len(nodeA.GetBlocks()), len(nodeB.GetBlocks()), len(nodeC.GetBlocks()))
	}

	// All nodes must have adopted A's chain
	if nodeB.GetBlocks()[6].Hash != nodeA.GetBlocks()[6].Hash || nodeC.GetBlocks()[6].Hash != nodeA.GetBlocks()[6].Hash {
		t.Errorf("Nodes did not converge to the same preferred chain hash")
	}

	// 14. Verify orphaned transactions (txC in Node C's orphaned branch) are restored to mempools
	pendingC := nodeC.GetPendingTransactions()
	found := false
	for _, tx := range pendingC {
		if tx.ID == txC.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Orphaned transaction txC was not restored to Node C pending pool")
	}
}

// Helpers
func mineBlockOnNode(n *node.Node) {
	txs := n.GetPendingTransactions()
	if len(txs) == 0 {
		dummy := faucetTx("dummy", 1.0)
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

var dummyCounter int64

func TestFaucetAndTransactionSecurity(t *testing.T) {
	bc := blockchain.NewBlockchain()
	alice, _ := wallet.NewWallet()
	bob, _ := wallet.NewWallet()

	// 1. Unsigned normal transaction is rejected
	txUnsignedNormal := transaction.Transaction{
		Sender:   alice.Address,
		Receiver: bob.Address,
		Amount:   10.0,
	}
	txUnsignedNormal.ID, _ = txUnsignedNormal.CalculateID()
	err := bc.AddTransaction(txUnsignedNormal)
	if err == nil {
		t.Error("Expected unsigned normal transaction to be rejected, got nil")
	}

	// 2. Unsigned faucet transaction is rejected
	txUnsignedFaucet := transaction.Transaction{
		Sender:   "faucet",
		Receiver: alice.Address,
		Amount:   100.0,
	}
	txUnsignedFaucet.ID, _ = txUnsignedFaucet.CalculateID()
	err = bc.AddTransaction(txUnsignedFaucet)
	if err == nil {
		t.Error("Expected unsigned faucet transaction to be rejected, got nil")
	}

	// 3. Fake faucet public key/signature is rejected
	// We create a transaction claiming to be faucet, but signed with Alice's key
	txFakeFaucet, _ := transaction.NewSignedSpecialTransaction(
		"faucet",
		alice.PrivateKey,
		alice.PublicKey,
		bob.Address,
		50.0,
	)
	err = bc.AddTransaction(txFakeFaucet)
	if err == nil {
		t.Error("Expected faucet transaction signed with Alice's key (fake public key) to be rejected, got nil")
	}

	// 4. Valid faucet signature is accepted
	txValidFaucet := faucetTx(alice.Address, 100.0)
	err = bc.AddTransaction(txValidFaucet)
	if err != nil {
		t.Errorf("Expected valid faucet transaction to be accepted, got: %v", err)
	}

	// 5. Modified faucet transaction is rejected
	txModifiedFaucet := faucetTx(alice.Address, 100.0)
	txModifiedFaucet.Amount = 200.0 // modify amount after signing
	err = bc.AddTransaction(txModifiedFaucet)
	if err == nil {
		t.Error("Expected modified faucet transaction to be rejected, got nil")
	}

	// Mine the valid faucet transaction so Alice has 100 coins
	blockData, _ := bc.CreatePendingBlock(10)
	bc.AddMinedBlock(blockData)

	// 6. Valid normal Ed25519 transaction is accepted
	txValidNormal, _ := transaction.NewSignedTransaction(alice, bob.Address, 40.0)
	err = bc.AddTransaction(txValidNormal)
	if err != nil {
		t.Errorf("Expected valid signed normal transaction to be accepted, got: %v", err)
	}

	// 7. Modified normal transaction is rejected
	txModifiedNormal, _ := transaction.NewSignedTransaction(alice, bob.Address, 30.0)
	txModifiedNormal.Amount = 50.0 // modify amount after signing
	err = bc.AddTransaction(txModifiedNormal)
	if err == nil {
		t.Error("Expected modified normal transaction to be rejected, got nil")
	}
}

func TestNetworkFaucetAndBlockSecurity(t *testing.T) {
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

	client := &http.Client{Timeout: 2 * time.Second}
	alice, _ := wallet.NewWallet()

	// 8. Invalid faucet transaction received through /transactions is rejected
	txInvalidFaucet := transaction.Transaction{
		Sender:   "faucet",
		Receiver: alice.Address,
		Amount:   1000.0,
	}
	txInvalidFaucet.ID, _ = txInvalidFaucet.CalculateID()
	data, _ := json.Marshal(txInvalidFaucet)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("POST /transactions failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t.Error("Expected POST /transactions with invalid faucet tx to be rejected, but it succeeded")
	}

	// 9. Invalid faucet transaction inside a received block is rejected
	invalidBlock := block.Block{
		Index:        1,
		Timestamp:    time.Now().Unix(),
		Transactions: []transaction.Transaction{txInvalidFaucet},
		PreviousHash: nodeA.GetBlocks()[0].Hash,
		Difficulty:   1,
	}
	invalidBlock.MerkleRoot = utils.CalculateMerkleRoot(invalidBlock.Transactions)
	toy_mining.MineBlock(&invalidBlock, 1)

	blockData, _ := json.Marshal(invalidBlock)
	respBlock, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/blocks", nodeA.Port), "application/json", bytes.NewBuffer(blockData))
	if err != nil {
		t.Fatalf("POST /blocks failed: %v", err)
	}
	defer respBlock.Body.Close()
	if respBlock.StatusCode == http.StatusCreated || respBlock.StatusCode == http.StatusOK {
		t.Error("Expected block with invalid faucet transaction to be rejected, but it succeeded")
	}

	// 10. Valid faucet transaction propagates between nodes
	validFaucetTx := faucetTx(alice.Address, 100.0)
	validData, _ := json.Marshal(validFaucetTx)
	respValid, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(validData))
	if err != nil {
		t.Fatalf("POST /transactions failed: %v", err)
	}
	_ = respValid.Body.Close()

	// Wait for propagation to Node B
	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Errorf("Valid faucet transaction failed to propagate to Node B: %v", err)
	}

	// Clear mempool
	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeB)

	// 11. /faucet creates a properly signed transaction and propagates it
	faucetPayload, _ := json.Marshal(map[string]interface{}{
		"receiver": alice.Address,
		"amount":   50.0,
	})
	respFaucet, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/faucet", nodeA.Port), "application/json", bytes.NewBuffer(faucetPayload))
	if err != nil {
		t.Fatalf("POST /faucet failed: %v", err)
	}
	defer respFaucet.Body.Close()
	if respFaucet.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created from /faucet, got %d", respFaucet.StatusCode)
	}

	// Verify propagation of this faucet transaction to Node B
	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Errorf("Transaction created via Node A /faucet failed to propagate to Node B: %v", err)
	}
}

func TestNetworkForkConvergenceThreeNodes(t *testing.T) {
	var err error
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

	// 1. Construct independent forks:
	// Node A: Genesis -> A1 -> A2
	// Node B: Genesis -> B1 -> B2 -> B3 -> B4
	// Node C: Genesis -> C1
	mineBlockOnNode(nodeA) // A1
	mineBlockOnNode(nodeA) // A2

	mineBlockOnNode(nodeB) // B1
	mineBlockOnNode(nodeB) // B2
	mineBlockOnNode(nodeB) // B3
	mineBlockOnNode(nodeB) // B4

	mineBlockOnNode(nodeC) // C1

	blocksA := nodeA.GetBlocks()
	blocksB := nodeB.GetBlocks()
	blocksC := nodeC.GetBlocks()

	// 2. Verify all three chains are independently valid
	if !blockchain.ValidateChain(blocksA) {
		t.Fatal("Node A chain failed initial validation")
	}
	if !blockchain.ValidateChain(blocksB) {
		t.Fatal("Node B chain failed initial validation")
	}
	if !blockchain.ValidateChain(blocksC) {
		t.Fatal("Node C chain failed initial validation")
	}

	// 3. Verify they have the same genesis block but different tips
	if blocksA[0].Hash != blocksB[0].Hash || blocksB[0].Hash != blocksC[0].Hash {
		t.Fatal("Nodes do not share the same genesis block")
	}
	if blocksA[len(blocksA)-1].Hash == blocksB[len(blocksB)-1].Hash {
		t.Fatal("Node A and Node B tip hashes are identical, expected different forks")
	}
	if blocksC[len(blocksC)-1].Hash == blocksB[len(blocksB)-1].Hash {
		t.Fatal("Node C and Node B tip hashes are identical, expected different forks")
	}

	// 4. Verify that B's chain is preferred over A's and C's according to existing rules
	if !blockchain.IsPreferredChain(blocksB, blocksA) {
		t.Fatal("Expected Node B's chain to be preferred over Node A's")
	}
	if !blockchain.IsPreferredChain(blocksB, blocksC) {
		t.Fatal("Expected Node B's chain to be preferred over Node C's")
	}

	// 5. Gather expected orphaned transactions to verify later
	// Node A had blocks A1 and A2 containing dummy transactions.
	var expectedOrphanedA []string
	for _, b := range blocksA[1:] {
		for _, tx := range b.Transactions {
			expectedOrphanedA = append(expectedOrphanedA, tx.ID)
		}
	}
	// Node C had block C1 containing a dummy transaction.
	var expectedOrphanedC []string
	for _, b := range blocksC[1:] {
		for _, tx := range b.Transactions {
			expectedOrphanedC = append(expectedOrphanedC, tx.ID)
		}
	}

	// 6. Trigger production synchronization
	peerB := node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port}
	err = nodeA.SyncWithPeer(peerB)
	if err != nil {
		t.Fatalf("Node A failed to sync with Node B: %v", err)
	}
	err = nodeC.SyncWithPeer(peerB)
	if err != nil {
		t.Fatalf("Node C failed to sync with Node B: %v", err)
	}

	// 7. Verify convergence on Node B's chain
	finalBlocksA := nodeA.GetBlocks()
	finalBlocksC := nodeC.GetBlocks()

	if len(finalBlocksA) != len(blocksB) {
		t.Errorf("Node A length mismatch: expected %d, got %d", len(blocksB), len(finalBlocksA))
	}
	if len(finalBlocksC) != len(blocksB) {
		t.Errorf("Node C length mismatch: expected %d, got %d", len(blocksB), len(finalBlocksC))
	}

	if finalBlocksA[len(finalBlocksA)-1].Hash != blocksB[len(blocksB)-1].Hash {
		t.Errorf("Node A head hash did not converge to Node B head hash")
	}
	if finalBlocksC[len(finalBlocksC)-1].Hash != blocksB[len(blocksB)-1].Hash {
		t.Errorf("Node C head hash did not converge to Node B head hash")
	}

	// Verify post-sync chains are valid
	if !blockchain.ValidateChain(finalBlocksA) {
		t.Error("Node A final chain is invalid")
	}
	if !blockchain.ValidateChain(finalBlocksC) {
		t.Error("Node C final chain is invalid")
	}

	// 8. Verify that orphaned transactions were restored to their mempools
	pendingA := nodeA.GetPendingTransactions()
	if len(pendingA) != len(expectedOrphanedA) {
		t.Errorf("Node A pending transactions count mismatch: expected %d, got %d", len(expectedOrphanedA), len(pendingA))
	} else {
		for _, expectedID := range expectedOrphanedA {
			found := false
			for _, tx := range pendingA {
				if tx.ID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Orphaned transaction %s was not restored to Node A's mempool", expectedID)
			}
		}
	}

	pendingC := nodeC.GetPendingTransactions()
	if len(pendingC) != len(expectedOrphanedC) {
		t.Errorf("Node C pending transactions count mismatch: expected %d, got %d", len(expectedOrphanedC), len(pendingC))
	} else {
		for _, expectedID := range expectedOrphanedC {
			found := false
			for _, tx := range pendingC {
				if tx.ID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Orphaned transaction %s was not restored to Node C's mempool", expectedID)
			}
		}
	}

	// 9. Verify invalid chain rejection during synchronization
	nodeD := node.NewNode("node-d", "127.0.0.1", 0, nil)
	_ = nodeD.StartServer()
	defer func() { _ = nodeD.Shutdown(context.Background()) }()

	// Sync Node D with B first to get the same valid chain
	err = nodeD.SyncWithPeer(peerB)
	if err != nil {
		t.Fatalf("Failed to sync Node D with Node B initially: %v", err)
	}

	// Mine one block on Node D
	mineBlockOnNode(nodeD)

	// Tamper with the mined block on Node D (make transaction amount invalid)
	blocksD := nodeD.GetBlocks()
	lastBlock := blocksD[len(blocksD)-1]
	lastBlock.Transactions[0].Amount = 9999.0
	lastBlock.MerkleRoot = utils.CalculateMerkleRoot(lastBlock.Transactions)
	lastBlock.Hash = utils.CalculateHash(lastBlock)
	nodeD.Blockchain.Blocks[len(blocksD)-1] = lastBlock

	// Try to sync Node A with Node D's invalid chain
	peerD := node.Peer{ID: nodeD.ID, Host: nodeD.Host, Port: nodeD.Port}
	err = nodeA.SyncWithPeer(peerD)
	if err == nil {
		t.Error("Expected SyncWithPeer to fail when syncing with invalid chain from Node D")
	}

	// Verify Node A's chain did not reorganize and remains unchanged
	if len(nodeA.GetBlocks()) != len(blocksB) {
		t.Errorf("Node A chain length changed after invalid sync attempt: expected %d, got %d", len(blocksB), len(nodeA.GetBlocks()))
	}
	if nodeA.GetBlocks()[len(nodeA.GetBlocks())-1].Hash != blocksB[len(blocksB)-1].Hash {
		t.Errorf("Node A tip hash changed after invalid sync attempt")
	}
}
