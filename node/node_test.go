package node_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"toy-blockchain/blockchain"
	"toy-blockchain/node"
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

	// 7 (Already Running Check). Verify calling StartServer() when already running returns error
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

func TestMultipleConcurrentNodes(t *testing.T) {
	// 6 (Multi-node). Multiple Nodes can use different ports without sharing one global server.
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
