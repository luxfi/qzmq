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

// XChainDEXTransport provides quantum-secure transport for X-Chain DEX operations
type XChainDEXTransport struct {
	mu       sync.RWMutex
	socket   Socket
	endpoint string
	opts     Options
}

// XChainDEXConfig configures the X-Chain DEX transport
type XChainDEXConfig struct {
	Endpoint      string
	EnableQuantum bool
	SecurityMode  string
}

// NewXChainDEXTransport creates a new X-Chain DEX transport
func NewXChainDEXTransport(config *XChainDEXConfig) (*XChainDEXTransport, error) {
	opts := DefaultOptions()
	if config.EnableQuantum {
		opts = ConservativeOptions()
	}

	transport, err := New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	socket, err := transport.NewSocket(DEALER)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	return &XChainDEXTransport{
		socket:   socket,
		endpoint: config.Endpoint,
		opts:     opts,
	}, nil
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
