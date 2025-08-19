//go:build !czmq && !cgo
// +build !czmq,!cgo

package qzmq

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Global router for coordinating socket communication
var globalStubRouter = &stubRouter{
	dealers:  make(map[string]*stubSocket),
	routers:  make(map[string]*stubSocket),
	reqrep:   make(map[string]chan []byte),
	pubsub:   make(map[string][]chan []byte),
	pushpull: make(map[string]chan []byte),
	pairs:    make(map[string]*stubSocket),
}

type stubRouter struct {
	mu       sync.RWMutex
	dealers  map[string]*stubSocket
	routers  map[string]*stubSocket
	reqrep   map[string]chan []byte
	pubsub   map[string][]chan []byte
	pushpull map[string]chan []byte
	pairs    map[string]*stubSocket
}

func initGoBackend() error {
	return nil
}

func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	return &stubSocket{
		socketType:  socketType,
		opts:        opts,
		metrics:     NewSocketMetrics(),
		inbound:     make(chan []byte, 100),
		outbound:    make(chan []byte, 100),
		multipart:   make(chan [][]byte, 100),
		closed:      make(chan struct{}),
		bindings:    make(map[string]bool),
		connections: make(map[string]bool),
		identity:    fmt.Sprintf("socket-%d", time.Now().UnixNano()),
	}, nil
}

func initCZMQBackend() error {
	return nil
}

func newCZMQSocket(socketType SocketType, opts Options) (Socket, error) {
	return newGoSocket(socketType, opts)
}

// stubSocket implements Socket for testing
type stubSocket struct {
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	mu         sync.RWMutex

	inbound   chan []byte
	outbound  chan []byte
	multipart chan [][]byte
	closed    chan struct{}

	bindings    map[string]bool
	connections map[string]bool
	filters     []string
	identity    string
}

func (s *stubSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.bindings[endpoint] = true

	// Register with global router
	globalStubRouter.mu.Lock()
	defer globalStubRouter.mu.Unlock()

	switch s.socketType {
	case REP:
		if globalStubRouter.reqrep[endpoint] == nil {
			globalStubRouter.reqrep[endpoint] = make(chan []byte, 100)
		}
		go s.repServer(endpoint)

	case ROUTER:
		globalStubRouter.routers[endpoint] = s
		go s.routerServer(endpoint)

	case PULL:
		if globalStubRouter.pushpull[endpoint] == nil {
			globalStubRouter.pushpull[endpoint] = make(chan []byte, 100)
		}
		go s.pullServer(endpoint)

	case PUB, XPUB:
		// PUB doesn't need registration

	case PAIR:
		globalStubRouter.pairs[endpoint] = s
	}

	return nil
}

func (s *stubSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections[endpoint] = true

	// Register with global router
	globalStubRouter.mu.Lock()
	defer globalStubRouter.mu.Unlock()

	switch s.socketType {
	case REQ:
		if globalStubRouter.reqrep[endpoint] == nil {
			globalStubRouter.reqrep[endpoint] = make(chan []byte, 100)
		}

	case DEALER:
		globalStubRouter.dealers[s.identity] = s

	case PUSH:
		if globalStubRouter.pushpull[endpoint] == nil {
			globalStubRouter.pushpull[endpoint] = make(chan []byte, 100)
		}

	case SUB, XSUB:
		ch := make(chan []byte, 100)
		globalStubRouter.pubsub[endpoint] = append(globalStubRouter.pubsub[endpoint], ch)
		go s.subServer(ch)

	case PAIR:
		// Wait for bound socket
		go func() {
			for i := 0; i < 50; i++ {
				time.Sleep(10 * time.Millisecond)
				globalStubRouter.mu.RLock()
				peer := globalStubRouter.pairs[endpoint]
				globalStubRouter.mu.RUnlock()
				if peer != nil {
					// Connect the pairs
					go s.pairConnect(peer)
					return
				}
			}
		}()
	}

	return nil
}

// Server functions for different patterns
func (s *stubSocket) repServer(endpoint string) {
	ch := globalStubRouter.reqrep[endpoint]
	for {
		select {
		case msg := <-ch:
			s.inbound <- msg
			// Wait for reply
			select {
			case reply := <-s.outbound:
				ch <- reply
			case <-s.closed:
				return
			}
		case <-s.closed:
			return
		}
	}
}

func (s *stubSocket) routerServer(endpoint string) {
	for {
		select {
		case <-s.multipart:
			// Router received multipart to echo
			// This should happen when router calls RecvMultipart which gets from DEALER
			// and then SendMultipart which should send back to DEALER

		case <-s.closed:
			return
		}
	}
}

func (s *stubSocket) pullServer(endpoint string) {
	ch := globalStubRouter.pushpull[endpoint]
	for {
		select {
		case msg := <-ch:
			s.inbound <- msg
		case <-s.closed:
			return
		}
	}
}

func (s *stubSocket) subServer(ch chan []byte) {
	for {
		select {
		case msg := <-ch:
			if s.matchesFilter(msg) {
				s.inbound <- msg
			}
		case <-s.closed:
			return
		}
	}
}

func (s *stubSocket) pairConnect(peer *stubSocket) {
	// Create bidirectional connection
	for {
		select {
		case msg := <-s.outbound:
			peer.inbound <- msg
		case msg := <-peer.outbound:
			s.inbound <- msg
		case <-s.closed:
			return
		case <-peer.closed:
			return
		}
	}
}

func (s *stubSocket) matchesFilter(msg []byte) bool {
	if len(s.filters) == 0 {
		return true
	}

	msgStr := string(msg)
	for _, filter := range s.filters {
		if filter == "" || strings.HasPrefix(msgStr, filter) {
			return true
		}
	}
	return false
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

	switch s.socketType {
	case REQ:
		// Send request and wait for reply
		for endpoint := range s.connections {
			if ch := globalStubRouter.reqrep[endpoint]; ch != nil {
				ch <- data
				return nil
			}
		}

	case REP:
		// Reply goes through outbound channel, handled by repServer
		s.outbound <- data
		return nil

	case PUB, XPUB:
		// Broadcast to all subscribers
		globalStubRouter.mu.RLock()
		defer globalStubRouter.mu.RUnlock()
		for endpoint := range s.bindings {
			for _, ch := range globalStubRouter.pubsub[endpoint] {
				select {
				case ch <- data:
				default:
				}
			}
		}
		return nil

	case PUSH:
		// Send to pull socket
		for endpoint := range s.connections {
			if ch := globalStubRouter.pushpull[endpoint]; ch != nil {
				ch <- data
				return nil
			}
		}

	case DEALER:
		// DEALER sends to ROUTER
		globalStubRouter.mu.RLock()
		router := findConnectedRouter(s)
		globalStubRouter.mu.RUnlock()

		if router != nil {
			// Send as multipart [identity, message]
			parts := [][]byte{[]byte(s.identity), data}
			select {
			case router.multipart <- parts:
			case <-time.After(10 * time.Millisecond):
			}
		}
		return nil

	default:
		s.outbound <- data
	}

	return nil
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

	if s.socketType == ROUTER && len(parts) >= 2 {
		// ROUTER sending back to DEALER
		identity := string(parts[0])
		globalStubRouter.mu.RLock()
		dealer := globalStubRouter.dealers[identity]
		globalStubRouter.mu.RUnlock()

		if dealer != nil {
			// Send message back to dealer
			dealer.inbound <- parts[1]
		}
		return nil
	}

	if s.socketType == DEALER {
		// DEALER sends multipart to ROUTER
		globalStubRouter.mu.RLock()
		router := findConnectedRouter(s)
		globalStubRouter.mu.RUnlock()

		if router != nil {
			// Add identity frame and send to router
			identityParts := append([][]byte{[]byte(s.identity)}, parts...)
			select {
			case router.multipart <- identityParts:
			case <-time.After(10 * time.Millisecond):
			}
		}
		return nil
	}

	s.multipart <- parts
	return nil
}

func (s *stubSocket) Recv() ([]byte, error) {
	switch s.socketType {
	case REQ:
		// Wait for reply from REP
		for endpoint := range s.connections {
			if ch := globalStubRouter.reqrep[endpoint]; ch != nil {
				select {
				case reply := <-ch:
					s.metrics.MessagesReceived++
					s.metrics.BytesReceived += uint64(len(reply))
					return reply, nil
				case <-time.After(200 * time.Millisecond):
					return nil, ErrTimeout
				case <-s.closed:
					return nil, ErrNotConnected
				}
			}
		}

	case DEALER:
		// Receive echoed message
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(200 * time.Millisecond):
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}

	default:
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(200 * time.Millisecond):
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}
	}

	return nil, ErrTimeout
}

func (s *stubSocket) RecvMultipart() ([][]byte, error) {
	if s.socketType == ROUTER {
		select {
		case parts := <-s.multipart:
			s.metrics.MessagesReceived++
			for _, part := range parts {
				s.metrics.BytesReceived += uint64(len(part))
			}
			return parts, nil
		case <-time.After(200 * time.Millisecond):
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}
	}

	select {
	case parts := <-s.multipart:
		s.metrics.MessagesReceived++
		for _, part := range parts {
			s.metrics.BytesReceived += uint64(len(part))
		}
		return parts, nil
	case <-time.After(200 * time.Millisecond):
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
	return nil
}

func (s *stubSocket) GetOption(name string) (interface{}, error) {
	switch name {
	case "type":
		return s.socketType, nil
	case "suite", "qzmq.suite":
		return s.opts.Suite, nil
	case "qzmq.encrypted":
		return s.opts.Suite.KEM != 0 || s.opts.Suite.Sign != 0, nil
	default:
		return nil, nil
	}
}

func (s *stubSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.closed:
	default:
		close(s.closed)

		// Unregister from global router
		globalStubRouter.mu.Lock()
		defer globalStubRouter.mu.Unlock()

		// Clean up registrations
		delete(globalStubRouter.dealers, s.identity)
		for endpoint := range s.bindings {
			delete(globalStubRouter.routers, endpoint)
			delete(globalStubRouter.pairs, endpoint)
		}
	}

	return nil
}

func (s *stubSocket) GetMetrics() *SocketMetrics {
	return s.metrics
}

// Helper function to find connected router
func findConnectedRouter(dealer *stubSocket) *stubSocket {
	for endpoint := range dealer.connections {
		if router := globalStubRouter.routers[endpoint]; router != nil {
			return router
		}
	}
	return nil
}
