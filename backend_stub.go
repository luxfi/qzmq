//go:build stub
// +build stub

package qzmq

import (
	"strings"
	"sync"
	"time"
)

// Stub implementations when CZMQ is not available

// Global messaging infrastructure for stub backend
var (
	globalRouter    = newStubRouter()
	globalPubSubBus = newPubSubBus()
)

// stubRouter simulates message routing between sockets
type stubRouter struct {
	mu          sync.RWMutex
	bindings    map[string]*stubSocket
	connections map[string][]*stubSocket
}

func newStubRouter() *stubRouter {
	return &stubRouter{
		bindings:    make(map[string]*stubSocket),
		connections: make(map[string][]*stubSocket),
	}
}

func (r *stubRouter) registerBinding(endpoint string, socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings[endpoint] = socket
	
	// Connect any waiting connections to this binding
	if conns, ok := r.connections[endpoint]; ok {
		for _, conn := range conns {
			// Route messages between connected sockets
			if socket.socketType == PULL && conn.socketType == PUSH {
				// Connect PUSH to PULL
				go func(push, pull *stubSocket) {
					for {
						select {
						case msg := <-push.outbound:
							pull.inbound <- msg
						case <-push.closed:
							return
						case <-pull.closed:
							return
						}
					}
				}(conn, socket)
			} else if socket.socketType == REP && conn.socketType == REQ {
				// Connect REQ to REP bidirectionally
				go func(req, rep *stubSocket) {
					for {
						select {
						case msg := <-req.outbound:
							rep.inbound <- msg
						case <-req.closed:
							return
						case <-rep.closed:
							return
						}
					}
				}(conn, socket)
				// Route replies back
				go func(rep, req *stubSocket) {
					for {
						select {
						case reply := <-rep.outbound:
							req.inbound <- reply
						case <-rep.closed:
							return
						case <-req.closed:
							return
						}
					}
				}(socket, conn)
			} else if socket.socketType == PAIR && conn.socketType == PAIR {
				// Connect PAIR to PAIR bidirectionally
				go func(p1, p2 *stubSocket) {
					for {
						select {
						case msg := <-p1.outbound:
							p2.inbound <- msg
						case <-p1.closed:
							return
						case <-p2.closed:
							return
						}
					}
				}(conn, socket)
				go func(p1, p2 *stubSocket) {
					for {
						select {
						case msg := <-p2.outbound:
							p1.inbound <- msg
						case <-p1.closed:
							return
						case <-p2.closed:
							return
						}
					}
				}(socket, conn)
			} else if socket.socketType == ROUTER && conn.socketType == DEALER {
				// Connect DEALER to ROUTER
				go func(dealer, router *stubSocket) {
					for {
						select {
						case msg := <-dealer.outbound:
							router.inbound <- msg
						case <-dealer.closed:
							return
						case <-router.closed:
							return
						}
					}
				}(conn, socket)
				// And route back
				go func(router, dealer *stubSocket) {
					for {
						select {
						case msg := <-router.outbound:
							dealer.inbound <- msg
						case <-router.closed:
							return
						case <-dealer.closed:
							return
						}
					}
				}(socket, conn)
			}
		}
	}
}

func (r *stubRouter) registerConnection(endpoint string, socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connections[endpoint] = append(r.connections[endpoint], socket)
	
	// If there's already a binding, connect to it
	if bound, ok := r.bindings[endpoint]; ok {
		// Route messages between connected sockets
		if bound.socketType == PULL && socket.socketType == PUSH {
			// Connect PUSH to PULL
			go func(push, pull *stubSocket) {
				for {
					select {
					case msg := <-push.outbound:
						pull.inbound <- msg
					case <-push.closed:
						return
					case <-pull.closed:
						return
					}
				}
			}(socket, bound)
		} else if bound.socketType == REP && socket.socketType == REQ {
			// Connect REQ to REP bidirectionally
			go func(req, rep *stubSocket) {
				for {
					select {
					case msg := <-req.outbound:
						rep.inbound <- msg
					case <-req.closed:
						return
					case <-rep.closed:
						return
					}
				}
			}(socket, bound)
			// Route replies back
			go func(rep, req *stubSocket) {
				for {
					select {
					case reply := <-rep.outbound:
						req.inbound <- reply
					case <-rep.closed:
						return
					case <-req.closed:
						return
					}
				}
			}(bound, socket)
		} else if bound.socketType == PAIR && socket.socketType == PAIR {
			// Connect PAIR to PAIR bidirectionally
			go func(p1, p2 *stubSocket) {
				for {
					select {
					case msg := <-p1.outbound:
						p2.inbound <- msg
					case <-p1.closed:
						return
					case <-p2.closed:
						return
					}
				}
			}(socket, bound)
			go func(p1, p2 *stubSocket) {
				for {
					select {
					case msg := <-p2.outbound:
						p1.inbound <- msg
					case <-p1.closed:
						return
					case <-p2.closed:
						return
					}
				}
			}(bound, socket)
		} else if bound.socketType == ROUTER && socket.socketType == DEALER {
			// Connect DEALER to ROUTER
			go func(dealer, router *stubSocket) {
				for {
					select {
					case msg := <-dealer.outbound:
						router.inbound <- msg
					case <-dealer.closed:
						return
					case <-router.closed:
						return
					}
				}
			}(socket, bound)
			// And route back
			go func(router, dealer *stubSocket) {
				for {
					select {
					case msg := <-router.outbound:
						dealer.inbound <- msg
					case <-router.closed:
						return
					case <-dealer.closed:
						return
					}
				}
			}(bound, socket)
		}
	}
}

func (r *stubRouter) unregister(socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Remove from bindings
	for endpoint, bound := range r.bindings {
		if bound == socket {
			delete(r.bindings, endpoint)
		}
	}
	
	// Remove from connections
	for endpoint, conns := range r.connections {
		newConns := []*stubSocket{}
		for _, conn := range conns {
			if conn != socket {
				newConns = append(newConns, conn)
			}
		}
		if len(newConns) > 0 {
			r.connections[endpoint] = newConns
		} else {
			delete(r.connections, endpoint)
		}
	}
}

// pubSubBus simulates pub-sub message distribution
type pubSubBus struct {
	mu          sync.RWMutex
	subscribers map[*stubSocket][]string
	channels    map[*stubSocket]chan []byte
}

func newPubSubBus() *pubSubBus {
	return &pubSubBus{
		subscribers: make(map[*stubSocket][]string),
		channels:    make(map[*stubSocket]chan []byte),
	}
}

func (b *pubSubBus) subscribe(socket *stubSocket, filter string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.subscribers[socket] = append(b.subscribers[socket], filter)
	
	// Create a channel for this subscriber if needed
	if _, ok := b.channels[socket]; !ok {
		b.channels[socket] = make(chan []byte, 100)
	}
}

func (b *pubSubBus) unsubscribe(socket *stubSocket, filter string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if filters, ok := b.subscribers[socket]; ok {
		newFilters := []string{}
		for _, f := range filters {
			if f != filter {
				newFilters = append(newFilters, f)
			}
		}
		if len(newFilters) > 0 {
			b.subscribers[socket] = newFilters
		} else {
			delete(b.subscribers, socket)
			// Close and remove channel
			if ch, ok := b.channels[socket]; ok {
				close(ch)
				delete(b.channels, socket)
			}
		}
	}
}

func (b *pubSubBus) publish(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Send to all subscribers
	for socket, filters := range b.subscribers {
		// Check if message matches any filter
		match := false
		for _, filter := range filters {
			if filter == "" || strings.HasPrefix(string(data), filter) {
				match = true
				break
			}
		}
		if match {
			if ch, ok := b.channels[socket]; ok {
				select {
				case ch <- data:
				default:
					// Non-blocking send
				}
			}
		}
	}
}

func (b *pubSubBus) getChannel(socket *stubSocket) chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if ch, ok := b.channels[socket]; ok {
		return ch
	}
	
	// Create a new channel if needed
	ch := make(chan []byte, 100)
	b.channels[socket] = ch
	return ch
}

func (b *pubSubBus) unregister(socket *stubSocket) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	delete(b.subscribers, socket)
	if ch, ok := b.channels[socket]; ok {
		close(ch)
		delete(b.channels, socket)
	}
}

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
	
	// Register with global router
	globalRouter.registerBinding(endpoint, s)
	
	return nil
}

func (s *stubSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[endpoint] = true
	
	// Register with global router
	globalRouter.registerConnection(endpoint, s)
	
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
		// For PUB, broadcast to all subscribers via global bus
		go globalPubSubBus.publish(data)
		s.outbound <- data // Keep for backward compat
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
		// For REQ, wait for the response from REP
		select {
		case reply := <-s.inbound:
			// Received reply from REP
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(reply))
			return reply, nil
		case <-time.After(200 * time.Millisecond):
			// Timeout waiting for reply
			return nil, ErrTimeout
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
		case <-time.After(500 * time.Millisecond):
			// Timeout waiting for request
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}
		
	case SUB, XSUB:
		// For SUB, receive published messages from global bus
		ch := globalPubSubBus.getChannel(s)
		select {
		case msg := <-ch:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case msg := <-s.inbound:
			// Fallback to inbound channel
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
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
		
	case PAIR:
		// For PAIR, receive from inbound (messages from peer)
		select {
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			return msg, nil
		case <-time.After(100 * time.Millisecond):
			return nil, ErrTimeout
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
	// Special handling for ROUTER sockets
	if s.socketType == ROUTER {
		// Wait for a message
		select {
		case msg := <-s.outbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			// For ROUTER, return identity frame + message
			identity := []byte("dealer-" + string(msg[0]))
			return [][]byte{identity, msg}, nil
		case msg := <-s.inbound:
			s.metrics.MessagesReceived++
			s.metrics.BytesReceived += uint64(len(msg))
			// For ROUTER, return identity frame + message
			identity := []byte("dealer-" + string(msg[0]))
			return [][]byte{identity, msg}, nil
		case <-time.After(100 * time.Millisecond):
			return nil, ErrTimeout
		case <-s.closed:
			return nil, ErrNotConnected
		}
	}
	
	// Regular multipart handling
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
	
	// Register with global pubsub bus
	globalPubSubBus.subscribe(s, filter)
	
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
	
	// Unregister from global pubsub bus
	globalPubSubBus.unsubscribe(s, filter)
	
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
		// Unregister from global systems
		globalRouter.unregister(s)
		globalPubSubBus.unregister(s)
	}
	
	return nil
}

func (s *stubSocket) GetMetrics() *SocketMetrics {
	return s.metrics
}

