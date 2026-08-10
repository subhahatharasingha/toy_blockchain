package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"toy-blockchain/blockchain"
)

// Node represents a single blockchain node in the network.
type Node struct {
	ID         string
	Host       string
	Port       int
	Blockchain *blockchain.Blockchain

	httpServer *http.Server
	mu         sync.Mutex
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

// StartServer starts the HTTP server on the configured Host and Port.
// If the server is already running, it returns an error.
func (n *Node) StartServer() error {
	n.mu.Lock()
	if n.httpServer != nil {
		n.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	addr := fmt.Sprintf("%s:%d", n.Host, n.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		n.mu.Unlock()
		return err
	}

	// Update the port if it was dynamically allocated (Port 0)
	if n.Port == 0 {
		n.Port = listener.Addr().(*net.TCPAddr).Port
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", n.handleHealth)
	mux.HandleFunc("/", n.handleHealth)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", n.Host, n.Port),
		Handler: mux,
	}
	n.httpServer = server
	n.mu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()

	return nil
}

// Shutdown cleanly stops the node's HTTP server.
// If the server is not running, it returns nil.
func (n *Node) Shutdown(ctx context.Context) error {
	n.mu.Lock()
	server := n.httpServer
	if server == nil {
		n.mu.Unlock()
		return nil
	}
	n.mu.Unlock()

	err := server.Shutdown(ctx)

	n.mu.Lock()
	n.httpServer = nil
	n.mu.Unlock()

	return err
}

// IsServerRunning returns true if the HTTP server is currently running.
func (n *Node) IsServerRunning() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.httpServer != nil
}

// handleHealth responds to health checks.
func (n *Node) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
