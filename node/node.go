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
	serverMu   sync.Mutex // Guards httpServer lifecycle and dynamic Port

	blockchainMu sync.Mutex // Guards Blockchain state (blocks, pending pool, forks)

	peers   map[string]Peer
	peersMu sync.RWMutex // Guards peers map configuration

	seenTxs      map[string]struct{}
	seenTxsMu    sync.RWMutex // Guards seen transaction IDs cache
	seenBlocks   map[string]struct{}
	seenBlocksMu sync.RWMutex // Guards seen block hashes cache
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
	n.serverMu.Lock()
	if n.httpServer != nil {
		n.serverMu.Unlock()
		return fmt.Errorf("server already running")
	}

	addr := fmt.Sprintf("%s:%d", n.Host, n.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		n.serverMu.Unlock()
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
	mux.HandleFunc("/chain", n.handleGetChain)
	mux.HandleFunc("/sync", n.handlePostSync)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", n.Host, n.Port),
		Handler: mux,
	}
	n.httpServer = server
	n.serverMu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()

	return nil
}

// Shutdown cleanly stops the node's HTTP server.
// If the server is not running, it returns nil.
func (n *Node) Shutdown(ctx context.Context) error {
	n.serverMu.Lock()
	server := n.httpServer
	if server == nil {
		n.serverMu.Unlock()
		return nil
	}
	n.serverMu.Unlock()

	err := server.Shutdown(ctx)

	n.serverMu.Lock()
	n.httpServer = nil
	n.serverMu.Unlock()

	return err
}

// IsServerRunning returns true if the HTTP server is currently running.
func (n *Node) IsServerRunning() bool {
	n.serverMu.Lock()
	defer n.serverMu.Unlock()
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

	n.serverMu.Lock()
	selfHost := n.Host
	selfPort := n.Port
	n.serverMu.Unlock()

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

// GetPeers returns a safe copy of all registered peers configuration.
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
	n.blockchainMu.Lock()
	n.Blockchain.AddMinedBlock(b)
	n.blockchainMu.Unlock()

	n.seenBlocksMu.Lock()
	n.seenBlocks[b.Hash] = struct{}{}
	n.seenBlocksMu.Unlock()

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

	n.serverMu.Lock()
	resp := nodeInfoResponse{
		ID:   n.ID,
		Host: n.Host,
		Port: n.Port,
	}
	n.serverMu.Unlock()

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

	// De-duplication check
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

	n.blockchainMu.Lock()
	err = n.Blockchain.AddTransaction(tx)
	n.blockchainMu.Unlock()

	if err != nil {
		// Remove from seen cache on validation error
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

	// De-duplication check
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

	n.blockchainMu.Lock()
	err = n.Blockchain.VerifyBlock(b)
	if err == nil {
		n.Blockchain.AddMinedBlock(b)
	}
	n.blockchainMu.Unlock()

	if err != nil {
		n.seenBlocksMu.Lock()
		delete(n.seenBlocks, b.Hash)
		n.seenBlocksMu.Unlock()

		// Trigger synchronization if block is out-of-order
		go n.SyncWithPeers()

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

// handleGetChain returns the node's current chain blocks as JSON.
func (n *Node) handleGetChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chain := n.GetBlocks()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(chain)
}

type syncRequest struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// handlePostSync requests synchronization with either a specific peer or all peers.
func (n *Node) handlePostSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req syncRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Host == "" || req.Port <= 0 {
		go n.SyncWithPeers()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","message":"Sync triggered with all configured peers"}`))
		return
	}

	targetPeer := Peer{
		ID:   req.ID,
		Host: req.Host,
		Port: req.Port,
	}

	go func() {
		_ = n.SyncWithPeer(targetPeer)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success","message":"Sync triggered with peer"}`))
}

// SyncWithPeers contacts all configured peers to synchronize blockchain states.
func (n *Node) SyncWithPeers() {
	peers := n.GetPeers()
	for _, p := range peers {
		go func(peer Peer) {
			_ = n.SyncWithPeer(peer)
		}(p)
	}
}

// SyncWithPeer synchronizes blockchain blocks with a single peer.
// Locks are NOT held while performing the network HTTP request to prevent blocking and deadlock scenarios.
func (n *Node) SyncWithPeer(p Peer) error {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://%s:%d/chain", p.Host, p.Port)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch chain from peer %s: %w", p.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned non-200 code: %d", p.ID, resp.StatusCode)
	}

	var peerBlocks []block.Block
	err = json.NewDecoder(resp.Body).Decode(&peerBlocks)
	if err != nil {
		return fmt.Errorf("failed to decode chain from peer %s: %w", p.ID, err)
	}

	n.blockchainMu.Lock()
	// Check if peer's chain is preferred over ours
	if !blockchain.IsPreferredChain(peerBlocks, n.Blockchain.Blocks) {
		n.blockchainMu.Unlock()
		return fmt.Errorf("peer %s chain is not preferred or invalid", p.ID)
	}

	// Perform reorganization and restore orphaned transactions to mempool
	err = n.Blockchain.Reorganize(peerBlocks)
	if err != nil {
		n.blockchainMu.Unlock()
		return fmt.Errorf("failed to reorganize local blockchain with peer %s chain: %w", p.ID, err)
	}

	// Read blocks and pending transactions while holding blockchainMu to update caches
	blocksCopy := make([]block.Block, len(n.Blockchain.Blocks))
	for i, b := range n.Blockchain.Blocks {
		txs := make([]transaction.Transaction, len(b.Transactions))
		copy(txs, b.Transactions)
		blocksCopy[i] = b
		blocksCopy[i].Transactions = txs
	}

	pendingCopy := make([]transaction.Transaction, len(n.Blockchain.PendingTransactions))
	copy(pendingCopy, n.Blockchain.PendingTransactions)

	n.blockchainMu.Unlock()

	// Update seen-block cache
	n.seenBlocksMu.Lock()
	n.seenBlocks = make(map[string]struct{})
	for _, b := range blocksCopy {
		n.seenBlocks[b.Hash] = struct{}{}
	}
	n.seenBlocksMu.Unlock()

	// Update seen-transaction cache
	n.seenTxsMu.Lock()
	n.seenTxs = make(map[string]struct{})
	for _, b := range blocksCopy {
		for _, tx := range b.Transactions {
			if tx.ID != "" {
				n.seenTxs[tx.ID] = struct{}{}
			}
		}
	}
	for _, tx := range pendingCopy {
		if tx.ID != "" {
			n.seenTxs[tx.ID] = struct{}{}
		}
	}
	n.seenTxsMu.Unlock()

	return nil
}

// GetPendingTransactions returns a safe copy of the pending transactions.
func (n *Node) GetPendingTransactions() []transaction.Transaction {
	n.blockchainMu.Lock()
	defer n.blockchainMu.Unlock()
	txs := make([]transaction.Transaction, len(n.Blockchain.PendingTransactions))
	copy(txs, n.Blockchain.PendingTransactions)
	return txs
}

// GetBlocks returns a safe, deep copy of the blocks including nested transaction slices.
func (n *Node) GetBlocks() []block.Block {
	n.blockchainMu.Lock()
	defer n.blockchainMu.Unlock()
	blocks := make([]block.Block, len(n.Blockchain.Blocks))
	for i, b := range n.Blockchain.Blocks {
		txs := make([]transaction.Transaction, len(b.Transactions))
		copy(txs, b.Transactions)
		blocks[i] = b
		blocks[i].Transactions = txs
	}
	return blocks
}

// AddTransaction adds a transaction to the node's blockchain in a thread-safe manner.
func (n *Node) AddTransaction(tx transaction.Transaction) error {
	n.blockchainMu.Lock()
	defer n.blockchainMu.Unlock()
	return n.Blockchain.AddTransaction(tx)
}

// CreatePendingBlock creates a block with pending transactions in a thread-safe manner.
func (n *Node) CreatePendingBlock(maxTx int) (block.Block, error) {
	n.blockchainMu.Lock()
	defer n.blockchainMu.Unlock()
	return n.Blockchain.CreatePendingBlock(maxTx)
}

// AddMinedBlock adds a mined block in a thread-safe manner.
func (n *Node) AddMinedBlock(b block.Block) {
	n.blockchainMu.Lock()
	defer n.blockchainMu.Unlock()
	n.Blockchain.AddMinedBlock(b)
}
