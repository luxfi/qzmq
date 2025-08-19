//go:build !stub && !czmq && !cgo
// +build !stub,!czmq,!cgo

package qzmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	zmq "github.com/luxfi/zmq/v4"
)

// pureSocket implements Socket using luxfi/zmq (pure Go)
type pureSocket struct {
	socket     zmq.Socket
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	mu         sync.RWMutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Initialize the pure Go backend (no-op for luxfi/zmq)
func initGoBackend() error {
	return nil
}

// Create a new pure Go socket using luxfi/zmq
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create the ZMQ socket based on type
	var socket zmq.Socket
	switch socketType {
	case REQ:
		socket = zmq.NewReq(ctx)
	case REP:
		socket = zmq.NewRep(ctx)
	case PUB:
		socket = zmq.NewPub(ctx)
	case SUB:
		socket = zmq.NewSub(ctx)
	case XPUB:
		socket = zmq.NewXPub(ctx)
	case XSUB:
		socket = zmq.NewXSub(ctx)
	case PUSH:
		socket = zmq.NewPush(ctx)
	case PULL:
		socket = zmq.NewPull(ctx)
	case PAIR:
		socket = zmq.NewPair(ctx)
	case DEALER:
		socket = zmq.NewDealer(ctx)
	case ROUTER:
		socket = zmq.NewRouter(ctx)
	default:
		cancel()
		return nil, fmt.Errorf("unsupported socket type: %v", socketType)
	}

	// Set socket options based on timeouts
	if opts.Timeouts.Handshake > 0 {
		socket.SetOption("sndtimeo", int(opts.Timeouts.Handshake/time.Millisecond))
		socket.SetOption("rcvtimeo", int(opts.Timeouts.Handshake/time.Millisecond))
	}
	if opts.Timeouts.Linger > 0 {
		socket.SetOption("linger", int(opts.Timeouts.Linger/time.Millisecond))
	}

	return &pureSocket{
		socket:     socket,
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (s *pureSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	return s.socket.Listen(endpoint)
}

func (s *pureSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	return s.socket.Dial(endpoint)
}

func (s *pureSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Skip encryption for now (testing mode)
	// TODO: Implement proper encryption once handshake is working

	// Send the message
	msg := zmq.NewMsg(data)
	err := s.socket.Send(msg)
	if err != nil {
		return err
	}

	// Update metrics
	s.metrics.MessagesSent++
	s.metrics.BytesSent += uint64(len(data))

	return nil
}

func (s *pureSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Skip encryption for now (testing mode)
	// TODO: Implement proper encryption once handshake is working

	// Create multipart message
	msg := zmq.NewMsgFrom(parts...)
	err := s.socket.SendMulti(msg)
	if err != nil {
		return err
	}

	// Update metrics
	s.metrics.MessagesSent++
	for _, part := range parts {
		s.metrics.BytesSent += uint64(len(part))
	}

	return nil
}

func (s *pureSocket) Recv() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrNotConnected
	}

	// Receive the message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}

	// Get the first frame (for single-part messages)
	if len(msg.Frames) == 0 {
		return nil, fmt.Errorf("received empty message")
	}
	data := msg.Frames[0]

	// Skip decryption for now (testing mode)
	// TODO: Implement proper decryption once handshake is working

	// Update metrics
	s.metrics.MessagesReceived++
	s.metrics.BytesReceived += uint64(len(data))

	return data, nil
}

func (s *pureSocket) RecvMultipart() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrNotConnected
	}

	// Receive the message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}

	// Get all frames
	parts := msg.Frames

	// Skip decryption for now (testing mode)
	// TODO: Implement proper decryption once handshake is working

	// Update metrics
	s.metrics.MessagesReceived++
	for _, part := range parts {
		s.metrics.BytesReceived += uint64(len(part))
	}

	return parts, nil
}

func (s *pureSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}

	return s.socket.SetOption(zmq.OptionSubscribe, filter)
}

func (s *pureSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}

	return s.socket.SetOption(zmq.OptionUnsubscribe, filter)
}

func (s *pureSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Pass through to underlying socket
	return s.socket.SetOption(name, value)
}

func (s *pureSocket) GetOption(name string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrNotConnected
	}

	// Handle our custom options
	switch name {
	case "type":
		return s.socketType, nil
	case "suite", "qzmq.suite":
		return s.opts.Suite, nil
	case "qzmq.encrypted":
		// Check if encryption is enabled
		return s.opts.Suite.KEM != 0 || s.opts.Suite.Sign != 0, nil
	default:
		// Pass through to underlying socket
		return s.socket.GetOption(name)
	}
}

func (s *pureSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.cancel()
	return s.socket.Close()
}

func (s *pureSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}

// Placeholder encryption/decryption functions
// These would be implemented with actual crypto in production
func (s *pureSocket) encrypt(data []byte) ([]byte, error) {
	// For now, return data as-is (encryption disabled for testing)
	return data, nil
}

func (s *pureSocket) decrypt(data []byte) ([]byte, error) {
	// For now, return data as-is (encryption disabled for testing)
	return data, nil
}