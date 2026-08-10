package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"toy-blockchain/block"
	"toy-blockchain/blockchain"
	"toy-blockchain/transaction"
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

	seenTxs      map[string]struct{}
	seenTxsMu    sync.RWMutex // guards seenTxs map
	seenBlocks   map[string]struct{}
	seenBlocksMu sync.RWMutex // guards seenBlocks map
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
		seenTxs:    make(map[string]struct{}),
		seenBlocks: make(map[string]struct{}),
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
	mux.HandleFunc("/transactions", n.handlePostTransaction)
	mux.HandleFunc("/blocks", n.handlePostBlock)

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

// AddMinedBlockAndGossip appends a mined block to the node's local blockchain and gossips it to peers.
func (n *Node) AddMinedBlockAndGossip(b block.Block) {
	n.mu.Lock()
	n.Blockchain.AddMinedBlock(b)
	n.seenBlocksMu.Lock()
	n.seenBlocks[b.Hash] = struct{}{}
	n.seenBlocksMu.Unlock()
	n.mu.Unlock()

	n.gossipBlock(b)
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

// handlePostTransaction receives a transaction and processes it for gossip.
func (n *Node) handlePostTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var tx transaction.Transaction
	err := json.NewDecoder(r.Body).Decode(&tx)
	if err != nil {
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}

	if tx.ID == "" || !tx.VerifyID() {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	// De-duplication: check if already seen
	n.seenTxsMu.Lock()
	if _, seen := n.seenTxs[tx.ID]; seen {
		n.seenTxsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","message":"Duplicate transaction ignored"}`))
		return
	}
	n.seenTxs[tx.ID] = struct{}{}
	n.seenTxsMu.Unlock()

	n.mu.Lock()
	err = n.Blockchain.AddTransaction(tx)
	n.mu.Unlock()

	if err != nil {
		// Clean up seen trace so it can be resubmitted if corrected
		n.seenTxsMu.Lock()
		delete(n.seenTxs, tx.ID)
		n.seenTxsMu.Unlock()

		http.Error(w, fmt.Sprintf("Invalid transaction: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"success","message":"Transaction accepted"}`))

	go n.gossipTransaction(tx)
}

// handlePostBlock receives a block and processes it for gossip.
func (n *Node) handlePostBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var b block.Block
	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}

	// De-duplication: check if already seen
	n.seenBlocksMu.Lock()
	if _, seen := n.seenBlocks[b.Hash]; seen {
		n.seenBlocksMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","message":"Duplicate block ignored"}`))
		return
	}
	n.seenBlocks[b.Hash] = struct{}{}
	n.seenBlocksMu.Unlock()

	n.mu.Lock()
	err = n.Blockchain.VerifyBlock(b)
	if err == nil {
		n.Blockchain.AddMinedBlock(b)
	}
	n.mu.Unlock()

	if err != nil {
		// Clean up seen trace
		n.seenBlocksMu.Lock()
		delete(n.seenBlocks, b.Hash)
		n.seenBlocksMu.Unlock()

		http.Error(w, fmt.Sprintf("Invalid block: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"status":"success","message":"Block accepted"}`))

	go n.gossipBlock(b)
}

func (n *Node) gossipTransaction(tx transaction.Transaction) {
	peers := n.GetPeers()
	if len(peers) == 0 {
		return
	}

	data, err := json.Marshal(tx)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}

	for _, p := range peers {
		go func(peer Peer) {
			url := fmt.Sprintf("http://%s:%d/transactions", peer.Host, peer.Port)
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_ = resp.Body.Close()
		}(p)
	}
}

func (n *Node) gossipBlock(b block.Block) {
	peers := n.GetPeers()
	if len(peers) == 0 {
		return
	}

	data, err := json.Marshal(b)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}

	for _, p := range peers {
		go func(peer Peer) {
			url := fmt.Sprintf("http://%s:%d/blocks", peer.Host, peer.Port)
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_ = resp.Body.Close()
		}(p)
	}
}

// GetPendingTransactions returns a copy of the pending transactions in a thread-safe manner.
func (n *Node) GetPendingTransactions() []transaction.Transaction {
	n.mu.Lock()
	defer n.mu.Unlock()
	txs := make([]transaction.Transaction, len(n.Blockchain.PendingTransactions))
	copy(txs, n.Blockchain.PendingTransactions)
	return txs
}

// GetBlocks returns a copy of the blocks in a thread-safe manner.
func (n *Node) GetBlocks() []block.Block {
	n.mu.Lock()
	defer n.mu.Unlock()
	blocks := make([]block.Block, len(n.Blockchain.Blocks))
	copy(blocks, n.Blockchain.Blocks)
	return blocks
}

// AddTransaction adds a transaction to the node's blockchain in a thread-safe manner.
func (n *Node) AddTransaction(tx transaction.Transaction) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.Blockchain.AddTransaction(tx)
}

// CreatePendingBlock creates a block with pending transactions in a thread-safe manner.
func (n *Node) CreatePendingBlock(maxTx int) (block.Block, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.Blockchain.CreatePendingBlock(maxTx)
}

// AddMinedBlock adds a mined block in a thread-safe manner.
func (n *Node) AddMinedBlock(b block.Block) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Blockchain.AddMinedBlock(b)
}


