// Package qzmq provides quantum-safe encryption for ZeroMQ
package qzmq

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Backend represents the underlying ZeroMQ implementation
type Backend int

const (
	// BackendAuto automatically selects the best available backend
	BackendAuto Backend = iota
	// BackendGo uses pure Go implementation (pebbe/zmq4)
	BackendGo
	// BackendCZMQ uses C bindings (goczmq)
	BackendCZMQ
)

// Mode represents the security mode
type Mode int

const (
	// ModeHybrid allows both classical and post-quantum algorithms
	ModeHybrid Mode = iota
	// ModePQOnly enforces post-quantum only algorithms
	ModePQOnly
	// ModeClassical uses only classical algorithms (not recommended)
	ModeClassical
)

// SocketType represents ZeroMQ socket types
type SocketType int

const (
	REQ    SocketType = iota // Request
	REP                      // Reply
	DEALER                   // Dealer
	ROUTER                   // Router
	PUB                      // Publisher
	SUB                      // Subscriber
	XPUB                     // Extended publisher
	XSUB                     // Extended subscriber
	PUSH                     // Push
	PULL                     // Pull
	PAIR                     // Pair
	STREAM                   // Stream
)

// Transport is the main QZMQ transport interface
type Transport interface {
	// NewSocket creates a new QZMQ-secured socket
	NewSocket(socketType SocketType) (Socket, error)

	// SetOption sets a transport-level option
	SetOption(name string, value interface{}) error

	// GetOption gets a transport-level option
	GetOption(name string) (interface{}, error)

	// Close closes the transport and all sockets
	Close() error

	// Stats returns transport statistics
	Stats() *Stats
}

// Socket represents a QZMQ-secured ZeroMQ socket
type Socket interface {
	// Bind binds the socket to an endpoint
	Bind(endpoint string) error

	// Connect connects the socket to an endpoint
	Connect(endpoint string) error

	// Send sends an encrypted message
	Send(data []byte) error

	// SendMultipart sends a multipart encrypted message
	SendMultipart(parts [][]byte) error

	// Recv receives and decrypts a message
	Recv() ([]byte, error)

	// RecvMultipart receives and decrypts a multipart message
	RecvMultipart() ([][]byte, error)

	// Subscribe sets a subscription filter (SUB sockets only)
	Subscribe(filter string) error

	// Unsubscribe removes a subscription filter
	Unsubscribe(filter string) error

	// SetOption sets a socket option
	SetOption(name string, value interface{}) error

	// GetOption gets a socket option
	GetOption(name string) (interface{}, error)

	// Close closes the socket
	Close() error

	// GetMetrics returns socket metrics
	GetMetrics() *SocketMetrics
}

// Options configures the QZMQ transport
type Options struct {
	// Backend selects the ZeroMQ implementation
	Backend Backend

	// Suite defines the cryptographic algorithms
	Suite Suite

	// Mode sets the security mode
	Mode Mode

	// KeyRotation configures automatic key rotation
	KeyRotation KeyRotationPolicy

	// AntiDoS enables DoS protection
	AntiDoS bool

	// ZeroRTT enables 0-RTT resumption
	ZeroRTT bool

	// MaxEarlyData limits 0-RTT data size
	MaxEarlyData uint32

	// Certificates for authentication
	Certificates *CertificateConfig

	// Timeouts for operations
	Timeouts TimeoutConfig
}

// Suite defines a cryptographic suite
type Suite struct {
	KEM  KemAlgorithm
	Sign SignatureAlgorithm
	AEAD AeadAlgorithm
	Hash HashAlgorithm
}

// KemAlgorithm represents key encapsulation mechanisms
type KemAlgorithm int

const (
	X25519 KemAlgorithm = iota
	MLKEM768
	MLKEM1024
	HybridX25519MLKEM768
	HybridX25519MLKEM1024
)

// SignatureAlgorithm represents signature algorithms
type SignatureAlgorithm int

const (
	Ed25519 SignatureAlgorithm = iota
	MLDSA2
	MLDSA3
)

// AeadAlgorithm represents AEAD algorithms
type AeadAlgorithm int

const (
	AES256GCM AeadAlgorithm = iota
	ChaCha20Poly1305
)

// HashAlgorithm represents hash algorithms
type HashAlgorithm int

const (
	SHA256 HashAlgorithm = iota
	SHA384
	SHA512
	BLAKE3
)

// KeyRotationPolicy defines when to rotate keys
type KeyRotationPolicy struct {
	MaxMessages uint64
	MaxBytes    uint64
	MaxAge      time.Duration
}

// CertificateConfig holds certificate settings
type CertificateConfig struct {
	CACert     []byte
	ClientCert []byte
	ClientKey  []byte
	ServerCert []byte
	ServerKey  []byte
	VerifyMode VerifyMode
}

// VerifyMode defines certificate verification
type VerifyMode int

const (
	VerifyNone VerifyMode = iota
	VerifyPeer
	VerifyFailIfNoPeerCert
)

// TimeoutConfig holds timeout settings
type TimeoutConfig struct {
	Handshake time.Duration
	Heartbeat time.Duration
	Linger    time.Duration
}

// Stats contains transport statistics
type Stats struct {
	HandshakesCompleted uint64
	HandshakesFailed    uint64
	MessagesEncrypted   uint64
	MessagesDecrypted   uint64
	BytesSent           uint64
	BytesReceived       uint64
	KeyRotations        uint64
	AuthFailures        uint64
	ReplayAttempts      uint64
}

// SocketMetrics contains per-socket metrics
type SocketMetrics struct {
	MessagesSent     uint64
	MessagesReceived uint64
	BytesSent        uint64
	BytesReceived    uint64
	Errors           uint64
	KeyUpdates       uint64
	AverageLatency   time.Duration
}

// transport implements the Transport interface
type transport struct {
	opts    Options
	backend Backend
	sockets map[string]Socket
	stats   *Stats
	mu      sync.RWMutex
	closed  bool
}

// New creates a new QZMQ transport with the specified options
func New(opts Options) (Transport, error) {
	// Validate options
	if err := validateOptions(&opts); err != nil {
		return nil, err
	}

	// Auto-detect backend if needed
	backend := opts.Backend
	if backend == BackendAuto {
		backend = detectBestBackend()
	}

	t := &transport{
		opts:    opts,
		backend: backend,
		sockets: make(map[string]Socket),
		stats:   &Stats{},
	}

	// Initialize the selected backend
	if err := t.initBackend(); err != nil {
		return nil, fmt.Errorf("failed to initialize backend: %w", err)
	}

	return t, nil
}

// DefaultOptions returns recommended default options
func DefaultOptions() Options {
	return Options{
		Backend: BackendAuto,
		Suite: Suite{
			KEM:  HybridX25519MLKEM768,
			Sign: MLDSA2,
			AEAD: AES256GCM,
			Hash: SHA256,
		},
		Mode: ModeHybrid,
		KeyRotation: KeyRotationPolicy{
			MaxMessages: 1 << 32, // 4 billion messages
			MaxBytes:    1 << 40, // 1 TB
			MaxAge:      10 * time.Minute,
		},
		AntiDoS:      true,
		ZeroRTT:      false,
		MaxEarlyData: 16384, // 16 KB
		Timeouts: TimeoutConfig{
			Handshake: 5 * time.Second,
			Heartbeat: 30 * time.Second,
			Linger:    0,
		},
	}
}

// ConservativeOptions returns maximum security options
func ConservativeOptions() Options {
	opts := DefaultOptions()
	opts.Suite = Suite{
		KEM:  MLKEM1024,
		Sign: MLDSA3,
		AEAD: AES256GCM,
		Hash: SHA384,
	}
	opts.Mode = ModePQOnly
	opts.ZeroRTT = false
	opts.KeyRotation.MaxMessages = 1 << 20 // 1 million
	opts.KeyRotation.MaxAge = 5 * time.Minute
	return opts
}

// PerformanceOptions returns high-performance options
func PerformanceOptions() Options {
	opts := DefaultOptions()
	opts.Suite = Suite{
		KEM:  X25519,
		Sign: Ed25519,
		AEAD: ChaCha20Poly1305,
		Hash: BLAKE3,
	}
	opts.Mode = ModeClassical
	opts.ZeroRTT = true
	opts.AntiDoS = false
	return opts
}

// NewSocket creates a new QZMQ-secured socket
func (t *transport) NewSocket(socketType SocketType) (Socket, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, errors.New("transport is closed")
	}

	// Create socket based on backend
	var socket Socket
	var err error

	switch t.backend {
	case BackendGo:
		socket, err = newGoSocket(socketType, t.opts)
	case BackendCZMQ:
		socket, err = newCZMQSocket(socketType, t.opts)
	default:
		return nil, fmt.Errorf("unsupported backend: %v", t.backend)
	}

	if err != nil {
		return nil, err
	}

	// Track socket
	socketID := fmt.Sprintf("%p", socket)
	t.sockets[socketID] = socket

	return socket, nil
}

// SetOption sets a transport-level option
func (t *transport) SetOption(name string, value interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch name {
	case "max_sockets":
		// Set maximum number of sockets
	case "io_threads":
		// Set I/O thread count
	default:
		return fmt.Errorf("unknown option: %s", name)
	}

	return nil
}

// GetOption gets a transport-level option
func (t *transport) GetOption(name string) (interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	switch name {
	case "backend":
		return t.backend, nil
	case "socket_count":
		return len(t.sockets), nil
	default:
		return nil, fmt.Errorf("unknown option: %s", name)
	}
}

// Close closes the transport and all sockets
func (t *transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	// Close all sockets
	for _, socket := range t.sockets {
		socket.Close()
	}

	t.sockets = nil
	t.closed = true

	return nil
}

// Stats returns transport statistics
func (t *transport) Stats() *Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return copy of stats
	statsCopy := *t.stats
	return &statsCopy
}

// initBackend initializes the selected backend
func (t *transport) initBackend() error {
	switch t.backend {
	case BackendGo:
		return initGoBackend()
	case BackendCZMQ:
		return initCZMQBackend()
	default:
		return fmt.Errorf("unsupported backend: %v", t.backend)
	}
}

// validateOptions validates the provided options
func validateOptions(opts *Options) error {
	if opts == nil {
		return errors.New("options cannot be nil")
	}

	// Validate suite
	if opts.Mode == ModePQOnly {
		switch opts.Suite.KEM {
		case X25519:
			return errors.New("classical KEM not allowed in PQ-only mode")
		}
		switch opts.Suite.Sign {
		case Ed25519:
			return errors.New("classical signatures not allowed in PQ-only mode")
		}
	}

	// Validate key rotation
	if opts.KeyRotation.MaxMessages == 0 {
		opts.KeyRotation.MaxMessages = 1 << 32
	}
	if opts.KeyRotation.MaxBytes == 0 {
		opts.KeyRotation.MaxBytes = 1 << 40
	}
	if opts.KeyRotation.MaxAge == 0 {
		opts.KeyRotation.MaxAge = 10 * time.Minute
	}

	return nil
}

// detectBestBackend auto-detects the best available backend
func detectBestBackend() Backend {
	// Try C backend first for performance
	if hasCZMQ() {
		return BackendCZMQ
	}
	// Fall back to pure Go
	return BackendGo
}

// hasCZMQ checks if CZMQ is available
func hasCZMQ() bool {
	// This would check if czmq bindings are available
	// For now, return false to use Go backend
	return false
}

// Errors
var (
	ErrInvalidOptions       = errors.New("invalid options")
	ErrBackendUnavailable   = errors.New("backend unavailable")
	ErrHandshakeFailed      = errors.New("handshake failed")
	ErrNotConnected         = errors.New("not connected")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrKeyRotationFailed    = errors.New("key rotation failed")
)
