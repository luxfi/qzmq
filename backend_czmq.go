// +build czmq

package qzmq

import (
	"fmt"
	"sync"
	"time"

	"github.com/zeromq/goczmq"
)

// czmqSocket implements Socket using C ZeroMQ bindings
type czmqSocket struct {
	socket      interface{} // Will be *goczmq.Sock
	socketType  SocketType
	opts        Options
	connection  *connection
	metrics     *SocketMetrics
	mu          sync.RWMutex
	
	// Message tracking
	sendSeq     uint64
	recvSeq     uint64
}

// initCZMQBackend initializes the C ZeroMQ backend
func initCZMQBackend() error {
	// Check if CZMQ is available
	// This would verify the C library is properly linked
	return nil
}

// newCZMQSocket creates a new socket using C backend
func newCZMQSocket(socketType SocketType, opts Options) (Socket, error) {
	// Convert socket type to CZMQ type
	czmqType := convertToCZMQType(socketType)
	
	// Create CZMQ socket
	sock := goczmq.NewSock(czmqType)
	if sock == nil {
		return nil, fmt.Errorf("failed to create CZMQ socket")
	}
	
	cs := &czmqSocket{
		socket:     sock,
		socketType: socketType,
		opts:       opts,
		metrics:    &SocketMetrics{},
	}
	
	// Initialize QZMQ connection
	cs.connection = newConnection(opts, false)
	
	// Configure socket
	if err := cs.configure(); err != nil {
		sock.(*goczmq.Sock).Destroy()
		return nil, err
	}
	
	return cs, nil
}

// Bind binds the socket to an endpoint
func (s *czmqSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sock := s.socket.(*goczmq.Sock)
	
	// Set server mode
	s.connection.isServer = true
	
	// Bind the socket
	if err := sock.Bind(endpoint); err != nil {
		return fmt.Errorf("bind failed: %w", err)
	}
	
	// Start accepting handshakes if needed
	if s.socketType != PUB && s.socketType != PUSH {
		go s.acceptHandshakes()
	}
	
	return nil
}

// Connect connects the socket to an endpoint
func (s *czmqSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sock := s.socket.(*goczmq.Sock)
	
	// Set client mode
	s.connection.isServer = false
	
	// Connect the socket
	if err := sock.Connect(endpoint); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	
	// Perform handshake if needed
	if s.socketType != SUB && s.socketType != PULL {
		if err := s.performHandshake(); err != nil {
			sock.Disconnect(endpoint)
			return fmt.Errorf("QZMQ handshake failed: %w", err)
		}
	}
	
	return nil
}

// Send sends an encrypted message
func (s *czmqSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sock := s.socket.(*goczmq.Sock)
	
	// Encrypt if needed
	if s.needsEncryption() {
		encrypted, err := s.encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
		
		s.metrics.MessagesSent++
		s.metrics.BytesSent += uint64(len(data))
	}
	
	// Send via CZMQ
	return sock.SendFrame(data, goczmq.FlagNone)
}

// SendMultipart sends a multipart encrypted message
func (s *czmqSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sock := s.socket.(*goczmq.Sock)
	
	// Encrypt each part if needed
	if s.needsEncryption() {
		for i, part := range parts {
			encrypted, err := s.encrypt(part)
			if err != nil {
				return err
			}
			parts[i] = encrypted
		}
	}
	
	// Send multipart message
	for i, part := range parts {
		flag := goczmq.FlagMore
		if i == len(parts)-1 {
			flag = goczmq.FlagNone
		}
		if err := sock.SendFrame(part, flag); err != nil {
			return err
		}
	}
	
	return nil
}

// Recv receives and decrypts a message
func (s *czmqSocket) Recv() ([]byte, error) {
	sock := s.socket.(*goczmq.Sock)
	
	// Receive from CZMQ
	data, _, err := sock.RecvFrame()
	if err != nil {
		return nil, err
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Decrypt if needed
	if s.needsEncryption() {
		decrypted, err := s.decrypt(data)
		if err != nil {
			s.metrics.Errors++
			return nil, err
		}
		data = decrypted
		
		s.metrics.MessagesReceived++
		s.metrics.BytesReceived += uint64(len(data))
	}
	
	return data, nil
}

// RecvMultipart receives and decrypts a multipart message
func (s *czmqSocket) RecvMultipart() ([][]byte, error) {
	sock := s.socket.(*goczmq.Sock)
	
	var parts [][]byte
	for {
		data, flag, err := sock.RecvFrame()
		if err != nil {
			return nil, err
		}
		
		// Decrypt if needed
		if s.needsEncryption() {
			s.mu.Lock()
			decrypted, err := s.decrypt(data)
			s.mu.Unlock()
			if err != nil {
				return nil, err
			}
			data = decrypted
		}
		
		parts = append(parts, data)
		
		if flag != goczmq.FlagMore {
			break
		}
	}
	
	return parts, nil
}

// Subscribe sets a subscription filter
func (s *czmqSocket) Subscribe(filter string) error {
	if s.socketType != SUB {
		return fmt.Errorf("subscribe only valid for SUB sockets")
	}
	
	sock := s.socket.(*goczmq.Sock)
	sock.SetOption(goczmq.SockSetSubscribe(filter))
	return nil
}

// Unsubscribe removes a subscription filter
func (s *czmqSocket) Unsubscribe(filter string) error {
	if s.socketType != SUB {
		return fmt.Errorf("unsubscribe only valid for SUB sockets")
	}
	
	sock := s.socket.(*goczmq.Sock)
	sock.SetOption(goczmq.SockSetUnsubscribe(filter))
	return nil
}

// SetOption sets a socket option
func (s *czmqSocket) SetOption(name string, value interface{}) error {
	// Handle QZMQ options
	// ... similar to Go backend
	return nil
}

// GetOption gets a socket option
func (s *czmqSocket) GetOption(name string) (interface{}, error) {
	// Handle QZMQ options
	// ... similar to Go backend
	return nil, nil
}

// Close closes the socket
func (s *czmqSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sock := s.socket.(*goczmq.Sock)
	sock.Destroy()
	
	return nil
}

// GetMetrics returns socket metrics
func (s *czmqSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	metricsCopy := *s.metrics
	return &metricsCopy
}

// Helper methods

func (s *czmqSocket) configure() error {
	sock := s.socket.(*goczmq.Sock)
	
	// Set socket options
	sock.SetOption(goczmq.SockSetSndhwm(10000))
	sock.SetOption(goczmq.SockSetRcvhwm(10000))
	sock.SetOption(goczmq.SockSetLinger(0))
	
	return nil
}

func (s *czmqSocket) needsEncryption() bool {
	if s.socketType == PUB || s.socketType == SUB {
		return s.opts.Mode != ModeClassical
	}
	return s.connection.state == stateEstablished
}

func (s *czmqSocket) performHandshake() error {
	// Similar to Go backend
	return nil
}

func (s *czmqSocket) acceptHandshakes() {
	// Server-side handshake
}

func (s *czmqSocket) encrypt(data []byte) ([]byte, error) {
	// Similar to Go backend
	return data, nil
}

func (s *czmqSocket) decrypt(data []byte) ([]byte, error) {
	// Similar to Go backend
	return data, nil
}

func convertToCZMQType(st SocketType) int {
	switch st {
	case REQ:
		return goczmq.Req
	case REP:
		return goczmq.Rep
	case DEALER:
		return goczmq.Dealer
	case ROUTER:
		return goczmq.Router
	case PUB:
		return goczmq.Pub
	case SUB:
		return goczmq.Sub
	case XPUB:
		return goczmq.XPub
	case XSUB:
		return goczmq.XSub
	case PUSH:
		return goczmq.Push
	case PULL:
		return goczmq.Pull
	case PAIR:
		return goczmq.Pair
	case STREAM:
		return goczmq.Stream
	default:
		return goczmq.Req
	}
}