package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"toy-blockchain/blockchain"
)

// Peer represents a configuration of another node in the network.
type Peer struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Node represents a single blockchain node in the network.
type Node struct {
	ID         string
	Host       string
	Port       int
	Blockchain *blockchain.Blockchain

	httpServer *http.Server
	mu         sync.Mutex // guards httpServer and Port

	peers   map[string]Peer
	peersMu sync.RWMutex // guards peers map
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
		peers:      make(map[string]Peer),
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
	mux.HandleFunc("/node", n.handleNodeInfo)
	mux.HandleFunc("/peers", n.handlePeers)

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

// AddPeer adds a peer configuration to the node.
func (n *Node) AddPeer(p Peer) error {
	if p.ID == "" {
		return fmt.Errorf("peer ID cannot be empty")
	}
	if p.Host == "" {
		return fmt.Errorf("peer host cannot be empty")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("invalid peer port: %d", p.Port)
	}

	if p.ID == n.ID {
		return fmt.Errorf("node cannot add itself as a peer")
	}

	n.mu.Lock()
	selfHost := n.Host
	selfPort := n.Port
	n.mu.Unlock()

	if p.Host == selfHost && p.Port == selfPort {
		return fmt.Errorf("node cannot add itself as a peer (matching host and port)")
	}

	n.peersMu.Lock()
	defer n.peersMu.Unlock()
	n.peers[p.ID] = p
	return nil
}

// RemovePeer removes a peer configuration from the node.
func (n *Node) RemovePeer(peerID string) error {
	n.peersMu.Lock()
	defer n.peersMu.Unlock()

	delete(n.peers, peerID)
	return nil
}

// GetPeers returns a copy of all registered peers configuration.
func (n *Node) GetPeers() []Peer {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()

	peersList := make([]Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peersList = append(peersList, p)
	}
	return peersList
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

type nodeInfoResponse struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type peersResponse struct {
	Peers []Peer `json:"peers"`
}

// handleNodeInfo returns node configuration as JSON.
func (n *Node) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n.mu.Lock()
	resp := nodeInfoResponse{
		ID:   n.ID,
		Host: n.Host,
		Port: n.Port,
	}
	n.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handlePeers returns current node peers as JSON.
func (n *Node) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := peersResponse{
		Peers: n.GetPeers(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
