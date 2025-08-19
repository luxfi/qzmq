//go:build stub
// +build stub

package qzmq

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

// stubBackend provides an in-memory implementation for testing without external dependencies
type stubBackend struct {
	mu      sync.RWMutex
	routers map[string]*stubSocket
	dealers map[string]*stubSocket
}

var globalStub = &stubBackend{
	routers: make(map[string]*stubSocket),
	dealers: make(map[string]*stubSocket),
}

// stubSocket implements Socket interface for testing
type stubSocket struct {
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	endpoint   string
	identity   string
	
	// Message channels
	incoming   chan [][]byte
	outgoing   chan [][]byte
	
	// Subscription filters (for SUB/XSUB)
	filters    map[string]bool
	
	// Connection state
	connected  bool
	bound      bool
	closed     bool
	
	mu         sync.RWMutex
}

// Initialize the stub backend (no-op)
func initGoBackend() error {
	return nil
}

// Create a new stub socket
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	s := &stubSocket{
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
		incoming:   make(chan [][]byte, 1000),
		outgoing:   make(chan [][]byte, 1000),
		filters:    make(map[string]bool),
	}
	
	// Generate default identity
	s.identity = fmt.Sprintf("%s-%d", socketType, time.Now().UnixNano())
	
	return s, nil
}

func (s *stubSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	s.endpoint = endpoint
	s.bound = true
	
	// Register with global router if ROUTER type
	if s.socketType == ROUTER {
		globalStub.mu.Lock()
		globalStub.routers[endpoint] = s
		globalStub.mu.Unlock()
	}
	
	// Start message router
	go s.routeMessages()
	
	return nil
}

func (s *stubSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	s.endpoint = endpoint
	s.connected = true
	
	// Register with global dealer if DEALER type
	if s.socketType == DEALER {
		globalStub.mu.Lock()
		globalStub.dealers[s.identity] = s
		globalStub.mu.Unlock()
	}
	
	// Start message router
	go s.routeMessages()
	
	return nil
}

func (s *stubSocket) Send(data []byte) error {
	return s.SendMultipart([][]byte{data})
}

func (s *stubSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Handle different socket types
	switch s.socketType {
	case REQ, REP:
		// Simple request-reply
		select {
		case s.outgoing <- parts:
			s.metrics.MessagesSent++
			for _, part := range parts {
				s.metrics.BytesSent += uint64(len(part))
			}
		case <-time.After(100 * time.Millisecond):
			return ErrTimeout
		}
		
	case DEALER:
		// Route to connected ROUTER
		globalStub.mu.RLock()
		router, ok := globalStub.routers[s.endpoint]
		globalStub.mu.RUnlock()
		
		if ok {
			// Add identity frame for ROUTER
			fullMsg := append([][]byte{[]byte(s.identity)}, parts...)
			select {
			case router.incoming <- fullMsg:
				s.metrics.MessagesSent++
				for _, part := range parts {
					s.metrics.BytesSent += uint64(len(part))
				}
			case <-time.After(100 * time.Millisecond):
				return ErrTimeout
			}
		}
		
	case ROUTER:
		// Route to specific DEALER
		if len(parts) < 1 {
			return fmt.Errorf("ROUTER requires identity frame")
		}
		identity := string(parts[0])
		payload := parts[1:]
		
		globalStub.mu.RLock()
		dealer, ok := globalStub.dealers[identity]
		globalStub.mu.RUnlock()
		
		if ok {
			select {
			case dealer.incoming <- payload:
				s.metrics.MessagesSent++
				for _, part := range parts {
					s.metrics.BytesSent += uint64(len(part))
				}
			case <-time.After(100 * time.Millisecond):
				return ErrTimeout
			}
		}
		
	case PUB:
		// Broadcast to all SUB sockets
		// In a real implementation, this would multicast
		s.metrics.MessagesSent++
		for _, part := range parts {
			s.metrics.BytesSent += uint64(len(part))
		}
		
	case PUSH:
		// Push to PULL socket
		select {
		case s.outgoing <- parts:
			s.metrics.MessagesSent++
			for _, part := range parts {
				s.metrics.BytesSent += uint64(len(part))
			}
		case <-time.After(100 * time.Millisecond):
			return ErrTimeout
		}
		
	default:
		// Generic send
		select {
		case s.outgoing <- parts:
			s.metrics.MessagesSent++
			for _, part := range parts {
				s.metrics.BytesSent += uint64(len(part))
			}
		case <-time.After(100 * time.Millisecond):
			return ErrTimeout
		}
	}
	
	return nil
}

func (s *stubSocket) Recv() ([]byte, error) {
	parts, err := s.RecvMultipart()
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return []byte{}, nil
	}
	return parts[0], nil
}

func (s *stubSocket) RecvMultipart() ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	select {
	case parts := <-s.incoming:
		s.metrics.MessagesReceived++
		for _, part := range parts {
			s.metrics.BytesReceived += uint64(len(part))
		}
		return parts, nil
		
	case <-time.After(100 * time.Millisecond):
		return nil, ErrTimeout
	}
}

func (s *stubSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}
	
	s.filters[filter] = true
	return nil
}

func (s *stubSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}
	
	delete(s.filters, filter)
	return nil
}

func (s *stubSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	switch name {
	case "identity":
		if id, ok := value.(string); ok {
			s.identity = id
			// Re-register with new identity
			if s.socketType == DEALER && s.connected {
				globalStub.mu.Lock()
				delete(globalStub.dealers, s.identity)
				globalStub.dealers[id] = s
				globalStub.mu.Unlock()
			}
		}
	case "linger", "sndhwm", "rcvhwm", "reconnect_ivl":
		// Accepted but ignored in stub
		return nil
	default:
		return fmt.Errorf("unsupported option: %s", name)
	}
	
	return nil
}

func (s *stubSocket) GetOption(name string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	switch name {
	case "type":
		return s.socketType, nil
	case "identity":
		return s.identity, nil
	case "suite", "qzmq.suite":
		return s.opts.Suite, nil
	case "qzmq.encrypted":
		return s.opts.Suite.KEM != 0 || s.opts.Suite.Sign != 0, nil
	default:
		return nil, fmt.Errorf("unsupported option: %s", name)
	}
}

func (s *stubSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	
	// Unregister from global registry
	if s.socketType == ROUTER && s.bound {
		globalStub.mu.Lock()
		delete(globalStub.routers, s.endpoint)
		globalStub.mu.Unlock()
	}
	if s.socketType == DEALER && s.connected {
		globalStub.mu.Lock()
		delete(globalStub.dealers, s.identity)
		globalStub.mu.Unlock()
	}
	
	// Close channels
	close(s.incoming)
	close(s.outgoing)
	
	return nil
}

func (s *stubSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	metrics := *s.metrics
	return &metrics
}

// routeMessages handles message routing for REQ-REP pattern
func (s *stubSocket) routeMessages() {
	for {
		s.mu.RLock()
		if s.closed {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()
		
		// Simple echo for REP sockets
		if s.socketType == REP {
			select {
			case msg := <-s.outgoing:
				// Echo back
				select {
				case s.incoming <- msg:
				case <-time.After(10 * time.Millisecond):
				}
			case <-time.After(10 * time.Millisecond):
			}
		}
		
		// Simple routing for REQ sockets
		if s.socketType == REQ {
			select {
			case msg := <-s.outgoing:
				// Simulate server response
				response := [][]byte{[]byte("response")}
				if len(msg) > 0 && bytes.Contains(msg[0], []byte("echo")) {
					response = msg
				}
				select {
				case s.incoming <- response:
				case <-time.After(10 * time.Millisecond):
				}
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}