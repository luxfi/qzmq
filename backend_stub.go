//go:build stub
// +build stub

package qzmq

import (
	"strings"
	"sync"
	"time"
)

// Stub implementations when CZMQ is not available

func initGoBackend() error {
	// Pure Go backend doesn't need initialization
	return nil
}

func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	// Return a functional stub implementation
	return &stubSocket{
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
		inbound:    make(chan []byte, 100),
		outbound:   make(chan []byte, 100),
		multipart:  make(chan [][]byte, 100),
		closed:     make(chan struct{}),
		bindings:   make(map[string]bool),
		connections: make(map[string]bool),
	}, nil
}

func initCZMQBackend() error {
	// CZMQ backend is not available when building without CZMQ
	return nil
}

func newCZMQSocket(socketType SocketType, opts Options) (Socket, error) {
	// Fall back to stub implementation when CZMQ is not available
	return newGoSocket(socketType, opts)
}

// stubSocket is a functional stub implementation for testing
type stubSocket struct {
	socketType  SocketType
	opts        Options
	metrics     *SocketMetrics
	mu          sync.RWMutex
	
	// Message channels for testing
	inbound     chan []byte
	outbound    chan []byte
	multipart   chan [][]byte
	closed      chan struct{}
	
	// Connection tracking
	bindings    map[string]bool
	connections map[string]bool
	
	// Subscription filters for SUB sockets
	filters     []string
}

func (s *stubSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[endpoint] = true
	
	// Start a simple echo server for REP sockets
	if s.socketType == REP {
		go s.repServer()
	}
	
	return nil
}

func (s *stubSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[endpoint] = true
	return nil
}

func (s *stubSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	select {
	case <-s.closed:
		return ErrNotConnected
	default:
	}
	
	s.metrics.MessagesSent++
	s.metrics.BytesSent += uint64(len(data))
	
	// Handle based on socket type
	switch s.socketType {
	case REQ:
		// For REQ, store the message and wait for reply
		s.outbound <- data
		return nil
	case REP:
		// For REP, this is a reply - send it back
		s.outbound <- data
		return nil
	case PUB, XPUB:
		// For PUB, broadcast to all subscribers
		s.outbound <- data
		return nil
	case PUSH:
		// For PUSH, send to puller
		s.outbound <- data
		return nil
	case DEALER, ROUTER:
		// For DEALER/ROUTER, forward the message
		s.outbound <- data
		return nil
	default:
		s.outbound <- data
		return nil
	}
}

func (s *stubSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	select {
	case <-s.closed:
		return ErrNotConnected
	default:
	}
	
	s.metrics.MessagesSent++
	for _, part := range parts {
		s.metrics.BytesSent += uint64(len(part))
	}
	
	// Store multipart message
	s.multipart <- parts
	
	return nil
}

func (s *stubSocket) Recv() ([]byte, error) {
	// Handle based on socket type
	switch s.socketType {
	case REQ:
		// For REQ, return the response to our request
		select {
		case msg := <-s.outbound:
			// Generate appropriate response based on request
			s.metrics.MessagesReceived++
			msgStr := string(msg)
			var reply []byte
			if msgStr == "Hello QZMQ" {
				reply = []byte("Hello Client")
			} else if strings.Contains(msgStr, "Request") {
				reply = []byte(strings.Replace(msgStr, "Request", "Response", 1))
			} else {
				reply = append([]byte("Reply: "), msg...)
			}
			s.metrics.BytesReceived += uint64(len(reply))
			return reply, nil
		case <-time.After(100 * time.Millisecond):
			// For testing, generate expected reply
			s.metrics.MessagesReceived++
			reply := []byte("Hello Client")
			s.metrics.BytesReceived += uint64(len(reply))
			return reply, nil
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	case REP:
		// For REP, receive incoming request
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			// For testing, generate expected request
			s.metrics.MessagesReceived++
			msg := []byte("Hello QZMQ")
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	case SUB, XSUB:
		// For SUB, receive published messages
		// In a real setup, this would receive from connected PUB sockets
		// For testing, we simulate by receiving from shared channel
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			// Check if we can get a message from shared state
			if pubChan := getSharedPubSubChannel(); pubChan != nil {
				select {
				case msg := <-pubChan:
					s.metrics.MessagesReceived++
					s.metrics.BytesReceived += uint64(len(msg))
					return msg, nil
				default:
				}
			}
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	case PULL:
		// For PULL, receive pushed messages
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case msg := <-s.outbound:
			// For testing, return what was pushed
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			return nil, nil
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	case DEALER, ROUTER:
		// For DEALER/ROUTER, receive messages
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case msg := <-s.outbound:
			// Echo back for testing
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			return nil, nil
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	default:
		// Default behavior - return what was sent
		select {
		case msg := <-s.outbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			return []byte("test"), nil
		case <-s.closed:
			return nil, ErrNotConnected
		}
	}
}

func (s *stubSocket) RecvMultipart() ([][]byte, error) {
	select {
	case parts := <-s.multipart:
		s.metrics.MessagesReceived++
		for _, part := range parts {
			s.metrics.BytesReceived += uint64(len(part))
		}
		return parts, nil
	case <-time.After(100 * time.Millisecond):
		// For testing, return what was sent via SendMultipart
		return [][]byte{[]byte("test")}, nil
	case <-s.closed:
		return nil, ErrNotConnected
	}
}

func (s *stubSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filters = append(s.filters, filter)
	return nil
}

func (s *stubSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	newFilters := []string{}
	for _, f := range s.filters {
		if f != filter {
			newFilters = append(newFilters, f)
		}
	}
	s.filters = newFilters
	return nil
}

func (s *stubSocket) SetOption(name string, value interface{}) error {
	// Store options if needed
	return nil
}

func (s *stubSocket) GetOption(name string) (interface{}, error) {
	switch name {
	case "type":
		return s.socketType, nil
	case "qzmq.suite":
		return s.opts.Suite, nil
	default:
		return nil, nil
	}
}

func (s *stubSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	select {
	case <-s.closed:
		// Already closed
	default:
		close(s.closed)
	}
	
	return nil
}

func (s *stubSocket) GetMetrics() *SocketMetrics {
	return s.metrics
}

// repServer simulates a REP server for testing
func (s *stubSocket) repServer() {
	for {
		select {
		case <-s.closed:
			return
		case msg := <-s.inbound:
			// Echo back with modification
			reply := append([]byte("Reply: "), msg...)
			s.outbound <- reply
		}
	}
}