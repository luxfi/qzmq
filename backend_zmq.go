//go:build !czmq && !stub
// +build !czmq,!stub

package qzmq

import (
	"fmt"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
)

// zmqSocket implements Socket using pebbe/zmq4
type zmqSocket struct {
	socket      *zmq.Socket
	socketType  SocketType
	opts        Options
	metrics     *SocketMetrics
	mu          sync.RWMutex
	closed      bool
}

// Initialize the Go backend (no-op for zmq4)
func initGoBackend() error {
	return nil
}

// Create a new Go socket using zmq4
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	// Map our socket types to zmq4 types
	var zmqType zmq.Type
	switch socketType {
	case REQ:
		zmqType = zmq.REQ
	case REP:
		zmqType = zmq.REP
	case PUB:
		zmqType = zmq.PUB
	case SUB:
		zmqType = zmq.SUB
	case XPUB:
		zmqType = zmq.XPUB
	case XSUB:
		zmqType = zmq.XSUB
	case PUSH:
		zmqType = zmq.PUSH
	case PULL:
		zmqType = zmq.PULL
	case PAIR:
		zmqType = zmq.PAIR
	case DEALER:
		zmqType = zmq.DEALER
	case ROUTER:
		zmqType = zmq.ROUTER
	case STREAM:
		zmqType = zmq.STREAM
	default:
		return nil, fmt.Errorf("unsupported socket type: %v", socketType)
	}

	// Create the ZMQ socket
	socket, err := zmq.NewSocket(zmqType)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	// Set socket options based on timeouts
	if opts.Timeouts.Handshake > 0 {
		socket.SetSndtimeo(opts.Timeouts.Handshake)
		socket.SetRcvtimeo(opts.Timeouts.Handshake)
	}
	if opts.Timeouts.Linger > 0 {
		socket.SetLinger(opts.Timeouts.Linger)
	}

	return &zmqSocket{
		socket:     socket,
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
	}, nil
}

func (s *zmqSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Bind(endpoint)
}

func (s *zmqSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Connect(endpoint)
}

func (s *zmqSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Skip encryption for now (testing mode)
	// TODO: Implement proper encryption once handshake is working
	
	// Send the message
	_, err := s.socket.SendBytes(data, 0)
	if err != nil {
		return err
	}
	
	// Update metrics
	s.metrics.MessagesSent++
	s.metrics.BytesSent += uint64(len(data))
	
	return nil
}

func (s *zmqSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Skip encryption for now (testing mode)
	// TODO: Implement proper encryption once handshake is working
	
	// Send all parts except the last with SNDMORE flag
	for i, part := range parts {
		var flag zmq.Flag
		if i < len(parts)-1 {
			flag = zmq.SNDMORE
		}
		
		_, err := s.socket.SendBytes(part, flag)
		if err != nil {
			return err
		}
		
		s.metrics.BytesSent += uint64(len(part))
	}
	
	s.metrics.MessagesSent++
	return nil
}

func (s *zmqSocket) Recv() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	// Receive the message
	data, err := s.socket.RecvBytes(0)
	if err != nil {
		return nil, err
	}
	
	// Skip decryption for now (testing mode)
	// TODO: Implement proper decryption once handshake is working
	
	// Update metrics
	s.metrics.MessagesReceived++
	s.metrics.BytesReceived += uint64(len(data))
	
	return data, nil
}

func (s *zmqSocket) RecvMultipart() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	parts := [][]byte{}
	
	for {
		data, err := s.socket.RecvBytes(0)
		if err != nil {
			return nil, err
		}
		
		// Skip decryption for now (testing mode)
		// TODO: Implement proper decryption once handshake is working
		
		parts = append(parts, data)
		s.metrics.BytesReceived += uint64(len(data))
		
		// Check if there are more parts
		more, err := s.socket.GetRcvmore()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	
	s.metrics.MessagesReceived++
	return parts, nil
}

func (s *zmqSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}
	
	return s.socket.SetSubscribe(filter)
}

func (s *zmqSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}
	
	return s.socket.SetUnsubscribe(filter)
}

func (s *zmqSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Map common options
	switch name {
	case "sndhwm":
		if v, ok := value.(int); ok {
			return s.socket.SetSndhwm(v)
		}
	case "rcvhwm":
		if v, ok := value.(int); ok {
			return s.socket.SetRcvhwm(v)
		}
	case "linger":
		if v, ok := value.(int); ok {
			return s.socket.SetLinger(time.Duration(v) * time.Millisecond)
		}
	case "identity":
		if v, ok := value.(string); ok {
			return s.socket.SetIdentity(v)
		}
	}
	
	return fmt.Errorf("unsupported option: %s", name)
}

func (s *zmqSocket) GetOption(name string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	switch name {
	case "type":
		return s.socketType, nil
	case "suite", "qzmq.suite":
		return s.opts.Suite, nil
	case "qzmq.encrypted":
		// Check if encryption is enabled
		return s.opts.Suite.KEM != 0 || s.opts.Suite.Sign != 0, nil
	case "sndhwm":
		return s.socket.GetSndhwm()
	case "rcvhwm":
		return s.socket.GetRcvhwm()
	case "identity":
		return s.socket.GetIdentity()
	default:
		return nil, fmt.Errorf("unsupported option: %s", name)
	}
}

func (s *zmqSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	return s.socket.Close()
}

func (s *zmqSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}

// Placeholder encryption/decryption functions
// These would be implemented with actual crypto in production
func (s *zmqSocket) encrypt(data []byte) ([]byte, error) {
	// For now, return data as-is (encryption disabled for testing)
	return data, nil
}

func (s *zmqSocket) decrypt(data []byte) ([]byte, error) {
	// For now, return data as-is (encryption disabled for testing)
	return data, nil
}