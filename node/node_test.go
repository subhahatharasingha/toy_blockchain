package node_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"toy-blockchain/blockchain"
	toy_mining "toy-blockchain/mining"
	"toy-blockchain/node"
	"toy-blockchain/transaction"
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
	// Allocate a dynamic port by setting Port to 0
	n := node.NewNode("test-node-lifecycle", "127.0.0.1", 0, nil)

	// Verify server is not running initially
	if n.IsServerRunning() {
		t.Fatal("Expected server not to be running initially")
	}

	// 1. Start server
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

	// 2. The server listens using the Node's configured address/port
	if n.Port == 0 {
		t.Fatal("Expected Port to be updated from 0 to the dynamically allocated port")
	}

	client := &http.Client{Timeout: 1 * time.Second}
	urlHealth := fmt.Sprintf("http://%s:%d/health", n.Host, n.Port)
	urlRoot := fmt.Sprintf("http://%s:%d/", n.Host, n.Port)

	// 3 & 4. Test health endpoints
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
			t.Fatalf("Failed to parse json body: %v. Body was: %s", err, string(bodyBytes))
		}

		if healthResp.Status != "OK" {
			t.Errorf("Expected status 'OK', got '%s'", healthResp.Status)
		}
	}

	// Verify calling StartServer() when already running returns error
	err = n.StartServer()
	if err == nil {
		t.Error("Expected error when starting an already running server, got nil")
	}

	// 5. Clean shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = n.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shut down server cleanly: %v", err)
	}

	if n.IsServerRunning() {
		t.Fatal("Expected IsServerRunning() to return false after shutdown")
	}

	// 6. Verify server is no longer accepting requests
	_, err = client.Get(urlHealth)
	if err == nil {
		t.Error("Expected request to fail after server shutdown, but it succeeded")
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

	client := &http.Client{Timeout: 1 * time.Second}
	url1 := fmt.Sprintf("http://127.0.0.1:%d/health", n1.Port)
	url2 := fmt.Sprintf("http://127.0.0.1:%d/health", n2.Port)

	resp1, err := client.Get(url1)
	if err != nil {
		t.Fatalf("Request to node 1 failed: %v", err)
	}
	_ = resp1.Body.Close()

	resp2, err := client.Get(url2)
	if err != nil {
		t.Fatalf("Request to node 2 failed: %v", err)
	}
	_ = resp2.Body.Close()

	if resp1.StatusCode != http.StatusOK || resp2.StatusCode != http.StatusOK {
		t.Error("Expected both nodes to return HTTP 200 OK")
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

	if peers[0].ID != p.ID || peers[0].Host != p.Host || peers[0].Port != p.Port {
		t.Errorf("Retrieved peer %+v does not match added peer %+v", peers[0], p)
	}
}

func TestAddDuplicatePeerNoDuplicates(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}

	_ = n.AddPeer(p)
	// Add identical peer again
	err := n.AddPeer(p)
	if err != nil {
		t.Fatalf("Expected adding duplicate peer to not fail, got error: %v", err)
	}

	// Add same peer ID with different Host/Port (updates configuration)
	pUpdated := node.Peer{ID: "peer-1", Host: "127.0.0.3", Port: 8002}
	err = n.AddPeer(pUpdated)
	if err != nil {
		t.Fatalf("Expected updating peer with same ID to succeed, got error: %v", err)
	}

	peers := n.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("Expected exactly 1 peer, got %d", len(peers))
	}
	if peers[0].Host != "127.0.0.3" || peers[0].Port != 8002 {
		t.Errorf("Expected peer config to be updated to 127.0.0.3:8002, got %s:%d", peers[0].Host, peers[0].Port)
	}
}

func TestNodeCannotAddSelfAsPeer(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)

	// 1. Same ID
	pSelfID := node.Peer{ID: "node-self", Host: "127.0.0.2", Port: 8001}
	err := n.AddPeer(pSelfID)
	if err == nil {
		t.Error("Expected error when adding self as peer (matching ID), got nil")
	}

	// 2. Same Host and Port
	pSelfAddr := node.Peer{ID: "peer-1", Host: "127.0.0.1", Port: 8000}
	err = n.AddPeer(pSelfAddr)
	if err == nil {
		t.Error("Expected error when adding self as peer (matching host/port), got nil")
	}
}

func TestPeerValidationRules(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)

	// Empty ID
	pEmptyID := node.Peer{ID: "", Host: "127.0.0.2", Port: 8001}
	if err := n.AddPeer(pEmptyID); err == nil {
		t.Error("Expected error for empty peer ID, got nil")
	}

	// Empty Host
	pEmptyHost := node.Peer{ID: "peer-1", Host: "", Port: 8001}
	if err := n.AddPeer(pEmptyHost); err == nil {
		t.Error("Expected error for empty peer Host, got nil")
	}

	// Port 0
	pPortZero := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 0}
	if err := n.AddPeer(pPortZero); err == nil {
		t.Error("Expected error for port 0, got nil")
	}

	// Negative Port
	pPortNeg := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: -80}
	if err := n.AddPeer(pPortNeg); err == nil {
		t.Error("Expected error for negative port, got nil")
	}

	// Too large Port
	pPortLarge := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 65536}
	if err := n.AddPeer(pPortLarge); err == nil {
		t.Error("Expected error for port > 65535, got nil")
	}
}

func TestRemovePeer(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}
	_ = n.AddPeer(p)

	// Remove existing peer
	err := n.RemovePeer("peer-1")
	if err != nil {
		t.Fatalf("Expected RemovePeer to succeed, got: %v", err)
	}

	if len(n.GetPeers()) != 0 {
		t.Error("Expected peers list to be empty after peer removal")
	}

	// Remove non-existing peer (should be handled cleanly)
	err = n.RemovePeer("peer-does-not-exist")
	if err != nil {
		t.Errorf("Expected RemovePeer of non-existent key to handle cleanly (no error), got: %v", err)
	}
}

func TestGetPeersImmutability(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}
	_ = n.AddPeer(p)

	peers := n.GetPeers()
	// Mutate retrieved peer slice and fields
	peers[0].ID = "mutated-id"
	peers[0].Host = "mutated-host"
	peers[0].Port = 9999

	// Verify internal state remains untouched
	originalPeers := n.GetPeers()
	if originalPeers[0].ID != "peer-1" || originalPeers[0].Host != "127.0.0.2" || originalPeers[0].Port != 8001 {
		t.Error("GetPeers() did not return a deep copy; internal state was mutated")
	}
}

func TestMultiplePeersStoredAndRetrievedIndependently(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	p1 := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}
	p2 := node.Peer{ID: "peer-2", Host: "127.0.0.3", Port: 8002}
	p3 := node.Peer{ID: "peer-3", Host: "127.0.0.4", Port: 8003}

	_ = n.AddPeer(p1)
	_ = n.AddPeer(p2)
	_ = n.AddPeer(p3)

	peers := n.GetPeers()
	if len(peers) != 3 {
		t.Fatalf("Expected exactly 3 peers, got %d", len(peers))
	}

	peerMap := make(map[string]node.Peer)
	for _, p := range peers {
		peerMap[p.ID] = p
	}

	for _, expected := range []node.Peer{p1, p2, p3} {
		retrieved, exists := peerMap[expected.ID]
		if !exists {
			t.Errorf("Expected to find peer %s, but it was missing", expected.ID)
		} else if retrieved.Host != expected.Host || retrieved.Port != expected.Port {
			t.Errorf("Peer %s retrieved config does not match expected", expected.ID)
		}
	}
}

func TestIndependentNodesMaintainIndependentPeers(t *testing.T) {
	n1 := node.NewNode("node-1", "127.0.0.1", 8000, nil)
	n2 := node.NewNode("node-2", "127.0.0.1", 8001, nil)

	p := node.Peer{ID: "peer-shared", Host: "127.0.0.2", Port: 8002}
	_ = n1.AddPeer(p)

	if len(n2.GetPeers()) != 0 {
		t.Error("Node 2 peer list should not be affected by changes to Node 1")
	}
}

func TestConcurrentPeerOperationsNoRaces(t *testing.T) {
	n := node.NewNode("node-self", "127.0.0.1", 8000, nil)
	done := make(chan bool)

	// Concurrent Add/Get/Remove
	go func() {
		for i := 0; i < 100; i++ {
			p := node.Peer{ID: fmt.Sprintf("peer-%d", i), Host: "127.0.0.2", Port: 8000 + i}
			_ = n.AddPeer(p)
			_ = n.GetPeers()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = n.RemovePeer(fmt.Sprintf("peer-%d", i))
			_ = n.GetPeers()
		}
		done <- true
	}()

	<-done
	<-done
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
	urlHealth := fmt.Sprintf("http://%s:%d/health", n.Host, n.Port)
	urlRoot := fmt.Sprintf("http://%s:%d/", n.Host, n.Port)

	// 1, 2, 3. Test GET /node
	respNode, err := client.Get(urlNode)
	if err != nil {
		t.Fatalf("Failed GET /node: %v", err)
	}
	if respNode.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for GET /node, got %d", respNode.StatusCode)
	}
	if respNode.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", respNode.Header.Get("Content-Type"))
	}
	var nodeInfo struct {
		ID   string `json:"id"`
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	bodyNode, _ := io.ReadAll(respNode.Body)
	_ = respNode.Body.Close()
	if err := json.Unmarshal(bodyNode, &nodeInfo); err != nil {
		t.Fatalf("Failed parsing GET /node JSON: %v", err)
	}
	if nodeInfo.ID != n.ID || nodeInfo.Host != n.Host || nodeInfo.Port != n.Port {
		t.Errorf("GET /node configuration mismatch: expected ID %s, Host %s, Port %d; got ID %s, Host %s, Port %d",
			n.ID, n.Host, n.Port, nodeInfo.ID, nodeInfo.Host, nodeInfo.Port)
	}

	// 4, 5, 6. Test GET /peers initially empty
	respPeers, err := client.Get(urlPeers)
	if err != nil {
		t.Fatalf("Failed GET /peers: %v", err)
	}
	if respPeers.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for GET /peers, got %d", respPeers.StatusCode)
	}
	var peersData struct {
		Peers []node.Peer `json:"peers"`
	}
	bodyPeers, _ := io.ReadAll(respPeers.Body)
	_ = respPeers.Body.Close()
	if err := json.Unmarshal(bodyPeers, &peersData); err != nil {
		t.Fatalf("Failed parsing GET /peers JSON: %v", err)
	}
	if len(peersData.Peers) != 0 {
		t.Errorf("Expected initially empty peer list, got %v", peersData.Peers)
	}

	// 7, 8. Test GET /peers after adding peers
	p1 := node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001}
	p2 := node.Peer{ID: "peer-2", Host: "127.0.0.3", Port: 8002}
	_ = n.AddPeer(p1)
	_ = n.AddPeer(p2)

	respPeers2, err := client.Get(urlPeers)
	if err != nil {
		t.Fatalf("Failed GET /peers: %v", err)
	}
	bodyPeers2, _ := io.ReadAll(respPeers2.Body)
	_ = respPeers2.Body.Close()
	if err := json.Unmarshal(bodyPeers2, &peersData); err != nil {
		t.Fatalf("Failed parsing GET /peers JSON: %v", err)
	}
	if len(peersData.Peers) != 2 {
		t.Fatalf("Expected 2 peers, got %d", len(peersData.Peers))
	}

	peerMap := make(map[string]node.Peer)
	for _, p := range peersData.Peers {
		peerMap[p.ID] = p
	}
	for _, expected := range []node.Peer{p1, p2} {
		retrieved, ok := peerMap[expected.ID]
		if !ok {
			t.Errorf("Expected peer %s not found in GET /peers response", expected.ID)
		} else if retrieved.Host != expected.Host || retrieved.Port != expected.Port {
			t.Errorf("Config mismatch for peer %s: expected %s:%d, got %s:%d",
				expected.ID, expected.Host, expected.Port, retrieved.Host, retrieved.Port)
		}
	}

	// 11. Test POST /node is rejected
	respPostNode, err := client.Post(urlNode, "application/json", nil)
	if err != nil {
		t.Fatalf("POST /node request failed: %v", err)
	}
	_ = respPostNode.Body.Close()
	if respPostNode.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed for POST /node, got %d", respPostNode.StatusCode)
	}

	// 12. Test POST /peers is rejected
	respPostPeers, err := client.Post(urlPeers, "application/json", nil)
	if err != nil {
		t.Fatalf("POST /peers request failed: %v", err)
	}
	_ = respPostPeers.Body.Close()
	if respPostPeers.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed for POST /peers, got %d", respPostPeers.StatusCode)
	}

	// 9 & 10. Health and / still work
	for _, url := range []string{urlHealth, urlRoot} {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("GET request failed to %s: %v", url, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected GET to %s to return 200, got %d", url, resp.StatusCode)
		}
	}
}

func TestConcurrentIntrospectionAPI(t *testing.T) {
	n := node.NewNode("concurrency-node", "127.0.0.1", 0, nil)
	_ = n.AddPeer(node.Peer{ID: "peer-1", Host: "127.0.0.2", Port: 8001})

	err := n.StartServer()
	if err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer func() {
		_ = n.Shutdown(context.Background())
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	urlNode := fmt.Sprintf("http://%s:%d/node", n.Host, n.Port)
	urlPeers := fmt.Sprintf("http://%s:%d/peers", n.Host, n.Port)

	done := make(chan bool)
	workers := 5

	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				resp, err := client.Get(urlNode)
				if err == nil {
					_ = resp.Body.Close()
				}
				resp2, err := client.Get(urlPeers)
				if err == nil {
					_ = resp2.Body.Close()
				}
			}
			done <- true
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
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

	// Seed Node A and B's blockchain with balance for sender
	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: aliceWallet.Address, Amount: 100.0}
	
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)

	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeB)

	// Create valid signed transaction
	tx, err := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}

	// Submit to Node A
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(tx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post to node A failed: %v", err)
	}
	_ = resp.Body.Close()

	// Wait for gossip to complete
	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Node B did not receive gossiped transaction: %v", err)
	}

	// Verify Node B has the transaction
	nodeBPending := nodeB.GetPendingTransactions()
	if len(nodeBPending) != 1 || nodeBPending[0].ID != tx.ID {
		t.Errorf("Node B pending transaction ID mismatch: expected %s, got %v", tx.ID, nodeBPending)
	}

	// Send same transaction to Node B again
	resp2, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeB.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post duplicate to node B failed: %v", err)
	}
	_ = resp2.Body.Close()

	// Wait a moment and confirm pending pool does not increase
	time.Sleep(100 * time.Millisecond)
	if len(nodeB.GetPendingTransactions()) != 1 {
		t.Errorf("Node B pending transactions pool increased on duplicate transaction; expected 1, got %d", len(nodeB.GetPendingTransactions()))
	}
}

func TestMultiNodeTransactionGossip(t *testing.T) {
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
	_ = nodeA.AddPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})
	_ = nodeB.AddPeer(node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port})
	_ = nodeC.AddPeer(node.Peer{ID: nodeA.ID, Host: nodeA.Host, Port: nodeA.Port})
	_ = nodeC.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	// Seed faucet balance on all nodes
	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: aliceWallet.Address, Amount: 100.0}
	
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)
	_ = nodeC.AddTransaction(faucetTx)

	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeB)
	mineBlockOnNode(nodeC)

	// Create valid signed transaction
	tx, err := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	if err != nil {
		t.Fatalf("Failed to create signed transaction: %v", err)
	}

	// Submit to Node A
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(tx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post to node A failed: %v", err)
	}
	_ = resp.Body.Close()

	// Wait for transaction to reach B and C
	err = waitForCondition(func() bool {
		return len(nodeB.GetPendingTransactions()) == 1 && len(nodeC.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("Transaction did not reach all nodes B and C: %v", err)
	}

	// Verify each node has exactly 1 copy of the transaction
	for _, n := range []*node.Node{nodeA, nodeB, nodeC} {
		pending := n.GetPendingTransactions()
		if len(pending) != 1 {
			t.Errorf("Node %s has invalid pending transaction count: %d", n.ID, len(pending))
		} else if pending[0].ID != tx.ID {
			t.Errorf("Node %s has mismatch transaction ID", n.ID)
		}
	}
}

func TestBlockGossip(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeB := node.NewNode("node-b", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeB.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeB.Shutdown(context.Background())
	}()

	_ = nodeA.AddPeer(node.Peer{ID: nodeB.ID, Host: nodeB.Host, Port: nodeB.Port})

	// Seed faucet balance on all nodes
	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()

	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: aliceWallet.Address, Amount: 100.0}
	
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeB.AddTransaction(faucetTx)

	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeB)

	// Create and add valid transaction on A
	tx, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	
	_ = nodeA.AddTransaction(tx)
	
	// Mine a block on Node A
	blockData, _ := nodeA.CreatePendingBlock(10)
	
	blockData.Timestamp = time.Now().Unix()
	toy_mining.MineBlock(&blockData, blockData.Difficulty)

	// Accept and Gossip it from Node A
	nodeA.AddMinedBlockAndGossip(blockData)

	// Verify Node B receives it
	err := waitForCondition(func() bool {
		return len(nodeB.GetBlocks()) == 3
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("Node B did not receive the block: %v", err)
	}

	// Verify same block received again does not create a duplicate
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(blockData)
	resp, _ := client.Post(fmt.Sprintf("http://127.0.0.1:%d/blocks", nodeB.Port), "application/json", bytes.NewBuffer(data))
	if resp != nil {
		_ = resp.Body.Close()
	}
	time.Sleep(100 * time.Millisecond)
	if len(nodeB.GetBlocks()) != 3 {
		t.Errorf("Node B created duplicate block; expected 3, got %d", len(nodeB.GetBlocks()))
	}
}

func TestInvalidDataRejection(t *testing.T) {
	n := node.NewNode("test-node", "127.0.0.1", 0, nil)
	_ = n.StartServer()
	defer func() { _ = n.Shutdown(context.Background()) }()

	client := &http.Client{Timeout: 1 * time.Second}

	// 1. Malformed JSON to POST /transactions
	resp, _ := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", n.Port), "application/json", bytes.NewBuffer([]byte("{invalid json")))
	if resp != nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for malformed JSON, got %d", resp.StatusCode)
		}
	}

	// 2. Invalid transaction ID
	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()
	tx, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	tx.ID = "invalid-hash-id"
	data, _ := json.Marshal(tx)
	resp, _ = client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", n.Port), "application/json", bytes.NewBuffer(data))
	if resp != nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for invalid ID, got %d", resp.StatusCode)
		}
	}

	// 3. Malformed JSON to POST /blocks
	resp, _ = client.Post(fmt.Sprintf("http://127.0.0.1:%d/blocks", n.Port), "application/json", bytes.NewBuffer([]byte("{invalid json")))
	if resp != nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for malformed JSON block, got %d", resp.StatusCode)
		}
	}
}

func TestPeerFailureGossip(t *testing.T) {
	nodeA := node.NewNode("node-a", "127.0.0.1", 0, nil)
	nodeC := node.NewNode("node-c", "127.0.0.1", 0, nil)

	_ = nodeA.StartServer()
	_ = nodeC.StartServer()
	defer func() {
		_ = nodeA.Shutdown(context.Background())
		_ = nodeC.Shutdown(context.Background())
	}()

	// Node B is offline (unavailable)
	peerB := node.Peer{ID: "node-b", Host: "127.0.0.1", Port: 54321}
	peerC := node.Peer{ID: nodeC.ID, Host: nodeC.Host, Port: nodeC.Port}

	_ = nodeA.AddPeer(peerB)
	_ = nodeA.AddPeer(peerC)

	// Seed faucet on A and C
	aliceWallet, _ := wallet.NewWallet()
	bobWallet, _ := wallet.NewWallet()
	faucetTx := transaction.Transaction{Sender: "faucet", Receiver: aliceWallet.Address, Amount: 100.0}
	
	_ = nodeA.AddTransaction(faucetTx)
	_ = nodeC.AddTransaction(faucetTx)

	mineBlockOnNode(nodeA)
	mineBlockOnNode(nodeC)

	// Submit transaction to A
	tx, _ := transaction.NewSignedTransaction(aliceWallet, bobWallet.Address, 10.0)
	client := &http.Client{Timeout: 1 * time.Second}
	data, _ := json.Marshal(tx)
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/transactions", nodeA.Port), "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	_ = resp.Body.Close()

	// Verify Node C receives it despite Node B being offline
	err = waitForCondition(func() bool {
		return len(nodeC.GetPendingTransactions()) == 1
	}, 3*time.Second)
	if err != nil {
		t.Errorf("Node C did not receive gossiped transaction when B was offline: %v", err)
	}

	// Node A must remain operational
	if !nodeA.IsServerRunning() {
		t.Error("Node A server stopped running or crashed")
	}
}

// Helpers for tests
func mineBlockOnNode(n *node.Node) {
	blockData, _ := n.CreatePendingBlock(10)
	blockData.Timestamp = time.Now().Unix()
	toy_mining.MineBlock(&blockData, blockData.Difficulty)
	n.AddMinedBlock(blockData)
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

