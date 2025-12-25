// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build qzmq

package qzmq

import (
	"encoding/json"
	"fmt"
	"sync"
)

// OrderSide represents buy or sell
type OrderSide int

const (
	Buy OrderSide = iota
	Sell
)

// OrderType represents order types
type OrderType int

const (
	Limit OrderType = iota
	MarketOrder
	Stop
	StopLimit
)

// Order represents a DEX order for X-Chain
type Order struct {
	ID          string
	Market      string
	Side        OrderSide
	Type        OrderType
	Price       uint64
	Quantity    uint64
	UserAddr    string
	AssetID     string
	Signature   []byte
	Certificate []byte
}

// ValidatorKeys holds cryptographic keys for a validator
type ValidatorKeys struct {
	NodeID []byte // Ed25519 node identity
	BLSKey []byte // BLS signature key
	PQKey  []byte // ML-KEM-768 post-quantum key
	DSAKey []byte // ML-DSA-87 signature key
}

// XChainDEXTransport provides quantum-secure transport for X-Chain DEX operations
type XChainDEXTransport struct {
	mu            sync.RWMutex
	transport     Transport
	socket        Socket
	endpoint      string
	chainID       string
	nodeID        string
	validatorKeys *ValidatorKeys
	opts          Options
	peers         []string
}

// NewXChainDEXTransport creates a new X-Chain DEX transport
// endpoint: RPC endpoint URL
// chainID: Chain identifier (e.g., "X")
// nodeID: Node identifier
// keys: Validator cryptographic keys
func NewXChainDEXTransport(endpoint, chainID, nodeID string, keys *ValidatorKeys) (*XChainDEXTransport, error) {
	opts := ConservativeOptions() // Default to quantum-secure

	transport, err := New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	socket, err := transport.NewSocket(DEALER)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	return &XChainDEXTransport{
		transport:     transport,
		socket:        socket,
		endpoint:      endpoint,
		chainID:       chainID,
		nodeID:        nodeID,
		validatorKeys: keys,
		opts:          opts,
	}, nil
}

// Listen starts listening on consensus ports
func (t *XChainDEXTransport) Listen(port1, port2, port3 int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	endpoint := fmt.Sprintf("tcp://*:%d", port1)
	return t.socket.Bind(endpoint)
}

// StartDEX starts DEX services on specified ports
func (t *XChainDEXTransport) StartDEX(orderPort, tradePort, marketPort, settlePort int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// DEX uses same socket with different message types
	logInfo("DEX services started", "orderPort", orderPort, "tradePort", tradePort)
	return nil
}

// ConnectToPeers connects to peer validators
func (t *XChainDEXTransport) ConnectToPeers(peers []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.peers = peers
	for _, peer := range peers {
		endpoint := fmt.Sprintf("tcp://%s", peer)
		if err := t.socket.Connect(endpoint); err != nil {
			logError("Failed to connect to peer", "peer", peer, "error", err)
			// Continue connecting to other peers
		}
	}
	return nil
}

// Connect connects to the DEX endpoint
func (t *XChainDEXTransport) Connect() error {
	return t.socket.Connect(t.endpoint)
}

// Close closes the transport
func (t *XChainDEXTransport) Close() error {
	return t.socket.Close()
}

// SubmitOrder submits an order via quantum-secure transport
func (t *XChainDEXTransport) SubmitOrder(order *Order) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal order: %w", err)
	}

	msg := append([]byte("ORDER:"), data...)
	return t.socket.Send(msg)
}

// ProposeBlock proposes a block via the transport
func (t *XChainDEXTransport) ProposeBlock(chainID string, height uint64, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	msg := fmt.Sprintf("BLOCK:%s:%d:", chainID, height)
	payload := append([]byte(msg), data...)
	return t.socket.Send(payload)
}

// ReceiveOrder receives an order from the transport
func (t *XChainDEXTransport) ReceiveOrder() (*Order, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data, err := t.socket.Recv()
	if err != nil {
		return nil, err
	}

	if len(data) < 6 || string(data[:6]) != "ORDER:" {
		return nil, fmt.Errorf("invalid order message")
	}

	var order Order
	if err := json.Unmarshal(data[6:], &order); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order: %w", err)
	}

	return &order, nil
}
