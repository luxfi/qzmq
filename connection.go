package qzmq

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

// Connection states
type connectionState int

const (
	stateInit connectionState = iota
	stateClientHello
	stateServerHello
	stateClientKey
	stateEstablished
	stateClosed
)

// Protocol constants
const (
	protocolVersion   = 1
	nonceSize         = 12
	defaultStreamID   = 1
	earlyDataStreamID = 0xFFFFFFFF
)

// connection manages QZMQ connection state
type connection struct {
	opts     Options
	isServer bool
	state    connectionState
	mu       sync.RWMutex

	// Handshake state
	clientRandom []byte
	serverRandom []byte
	transcript   []byte
	sessionID    []byte

	// Cryptographic state
	kem          KEM
	localKEMSK   PrivateKey
	localKEMPK   PublicKey
	remoteKEMPK  PublicKey
	clientAEAD   cipher.AEAD
	serverAEAD   cipher.AEAD
	keys         *keySet
	sharedSecret []byte

	// Session resumption
	psk          []byte
	earlyDataKey []byte

	// Key rotation
	lastKeyUpdate time.Time
	messageCount  uint64
	byteCount     uint64

	// Anti-DoS
	cookie     []byte
	cookieTime time.Time

	// Stream management
	streamID uint32
}

// newConnection creates a new QZMQ connection
func newConnection(opts Options, isServer bool) *connection {
	return &connection{
		opts:          opts,
		isServer:      isServer,
		state:         stateInit,
		streamID:      defaultStreamID,
		lastKeyUpdate: time.Now(),
	}
}

// clientHello generates ClientHello message
func (c *connection) clientHello() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != stateInit {
		return nil, errors.New("invalid state for ClientHello")
	}

	// Generate client random
	c.clientRandom = make([]byte, 32)
	if _, err := rand.Read(c.clientRandom); err != nil {
		return nil, err
	}

	// Initialize KEM
	c.kem = c.getKEM()

	// Generate ephemeral keys if not PQ-only
	if c.opts.Mode != ModePQOnly {
		var err error
		c.localKEMPK, c.localKEMSK, err = c.kem.GenerateKeyPair()
		if err != nil {
			return nil, err
		}
	}

	// Build ClientHello
	msg := &clientHelloMsg{
		version:   protocolVersion,
		random:    c.clientRandom,
		suite:     c.opts.Suite,
		sessionID: c.sessionID,
	}

	// Add extensions
	if c.opts.Mode == ModePQOnly {
		msg.extensions = append(msg.extensions, extension{
			typ:  extPQOnly,
			data: []byte{0x01},
		})
	}

	if c.localKEMPK != nil {
		msg.extensions = append(msg.extensions, extension{
			typ:  extKeyShare,
			data: c.localKEMPK.Bytes(),
		})
	}

	// Marshal and update transcript
	data := msg.marshal()
	c.transcript = append(c.transcript, data...)
	c.state = stateClientHello

	return data, nil
}

// processServerHello processes ServerHello message
func (c *connection) processServerHello(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != stateClientHello {
		return errors.New("invalid state for ServerHello")
	}

	msg := &serverHelloMsg{}
	if err := msg.unmarshal(data); err != nil {
		return err
	}

	// Store server random
	c.serverRandom = msg.random

	// Validate suite
	if msg.suite != c.opts.Suite {
		return errors.New("suite mismatch")
	}

	// Store server's KEM public key
	c.remoteKEMPK = &publicKey{data: msg.kemPublicKey}

	// TODO: Verify server certificate with ML-DSA

	// Update transcript
	c.transcript = append(c.transcript, data...)
	c.state = stateServerHello

	return nil
}

// clientKey generates ClientKey message
func (c *connection) clientKey() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != stateServerHello {
		return nil, errors.New("invalid state for ClientKey")
	}

	// Encapsulate to server's public key
	ciphertext, kemSecret, err := c.kem.Encapsulate(c.remoteKEMPK)
	if err != nil {
		return nil, err
	}

	// For hybrid mode, combine with ECDHE
	var ecdheSecret []byte
	if c.opts.Mode == ModeHybrid && c.localKEMSK != nil {
		// In real implementation, perform ECDHE with server's X25519 key
		ecdheSecret = make([]byte, 32)
		if _, err := rand.Read(ecdheSecret); err != nil {
			return nil, err
		}
	}

	// Derive keys
	c.keys, err = deriveKeys(kemSecret, ecdheSecret, c.opts.Suite.Hash)
	if err != nil {
		return nil, err
	}

	// Initialize AEADs
	c.clientAEAD, err = createAEAD(c.opts.Suite.AEAD, c.keys.clientKey)
	if err != nil {
		return nil, err
	}

	c.serverAEAD, err = createAEAD(c.opts.Suite.AEAD, c.keys.serverKey)
	if err != nil {
		return nil, err
	}

	// Build ClientKey message
	msg := &clientKeyMsg{
		kemCiphertext: ciphertext,
	}

	// TODO: Add client certificate and signature if mutual auth

	// Marshal and update transcript
	data := msg.marshal()
	c.transcript = append(c.transcript, data...)
	c.state = stateClientKey

	return data, nil
}

// processFinished processes Finished message
func (c *connection) processFinished(encData []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != stateClientKey {
		return errors.New("invalid state for Finished")
	}

	// Decrypt Finished message
	nonce := make([]byte, nonceSize)
	binary.BigEndian.PutUint32(nonce[0:4], c.streamID)
	binary.BigEndian.PutUint64(nonce[4:12], 0) // First encrypted message

	plaintext, err := c.serverAEAD.Open(nil, nonce, encData, nil)
	if err != nil {
		return errors.New("failed to decrypt Finished")
	}

	// Verify finished MAC
	expectedMAC := computeFinishedMAC(c.keys.serverKey, c.transcript)
	if len(plaintext) < 32 || string(plaintext[:32]) != string(expectedMAC) {
		return errors.New("finished MAC verification failed")
	}

	c.state = stateEstablished
	c.sharedSecret = c.keys.exporterSecret

	return nil
}

// needsKeyUpdate checks if key rotation is needed
func (c *connection) needsKeyUpdate() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.opts.KeyRotation.MaxMessages > 0 && c.messageCount >= c.opts.KeyRotation.MaxMessages {
		return true
	}

	if c.opts.KeyRotation.MaxBytes > 0 && c.byteCount >= c.opts.KeyRotation.MaxBytes {
		return true
	}

	if c.opts.KeyRotation.MaxAge > 0 && time.Since(c.lastKeyUpdate) > c.opts.KeyRotation.MaxAge {
		return true
	}

	return false
}

// updateKeys performs key rotation
func (c *connection) updateKeys() (*keySet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Generate new key material
	newSecret := make([]byte, 32)
	if _, err := rand.Read(newSecret); err != nil {
		return nil, err
	}

	// Derive new keys
	newKeys, err := deriveKeys(newSecret, c.sharedSecret, c.opts.Suite.Hash)
	if err != nil {
		return nil, err
	}

	// Reset counters
	c.messageCount = 0
	c.byteCount = 0
	c.lastKeyUpdate = time.Now()

	return newKeys, nil
}

// getKEM returns the appropriate KEM implementation
func (c *connection) getKEM() KEM {
	switch c.opts.Suite.KEM {
	case X25519:
		return &X25519KEM{}
	case MLKEM768:
		return &MLKEM768Type{}
	case MLKEM1024:
		return &MLKEM1024Type{}
	case HybridX25519MLKEM768:
		return NewHybridKEM(&X25519KEM{}, &MLKEM768Type{})
	case HybridX25519MLKEM1024:
		return NewHybridKEM(&X25519KEM{}, &MLKEM1024Type{})
	default:
		return &X25519KEM{}
	}
}

// Message types

type clientHelloMsg struct {
	version    uint8
	random     []byte
	suite      Suite
	sessionID  []byte
	extensions []extension
}

func (m *clientHelloMsg) marshal() []byte {
	// Simplified marshaling
	size := 1 + 32 + 4 + 1 + len(m.sessionID) + 2
	for _, ext := range m.extensions {
		size += 4 + len(ext.data)
	}

	buf := make([]byte, size)
	offset := 0

	buf[offset] = m.version
	offset++

	copy(buf[offset:], m.random)
	offset += 32

	// Marshal suite (simplified)
	offset += 4

	buf[offset] = byte(len(m.sessionID))
	offset++
	copy(buf[offset:], m.sessionID)
	offset += len(m.sessionID)

	// Extensions
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(m.extensions)))
	offset += 2

	for _, ext := range m.extensions {
		binary.BigEndian.PutUint16(buf[offset:], ext.typ)
		offset += 2
		binary.BigEndian.PutUint16(buf[offset:], uint16(len(ext.data)))
		offset += 2
		copy(buf[offset:], ext.data)
		offset += len(ext.data)
	}

	return buf[:offset]
}

type serverHelloMsg struct {
	version      uint8
	random       []byte
	suite        Suite
	sessionID    []byte
	kemPublicKey []byte
	certificate  []byte
	signature    []byte
	cookie       []byte
	extensions   []extension
}

func (m *serverHelloMsg) unmarshal(data []byte) error {
	if len(data) < 34 {
		return errors.New("ServerHello too short")
	}

	offset := 0
	m.version = data[offset]
	offset++

	m.random = make([]byte, 32)
	copy(m.random, data[offset:])
	offset += 32

	// Parse suite (simplified)
	offset += 4

	// Parse remaining fields...

	return nil
}

type clientKeyMsg struct {
	kemCiphertext []byte
	pskBinder     []byte
	clientSig     []byte
}

func (m *clientKeyMsg) marshal() []byte {
	size := 2 + len(m.kemCiphertext)
	if len(m.pskBinder) > 0 {
		size += 2 + len(m.pskBinder)
	}
	if len(m.clientSig) > 0 {
		size += 2 + len(m.clientSig)
	}

	buf := make([]byte, size)
	offset := 0

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(m.kemCiphertext)))
	offset += 2
	copy(buf[offset:], m.kemCiphertext)

	return buf
}

type extension struct {
	typ  uint16
	data []byte
}

// Extension types
const (
	extKeyShare  uint16 = 0x0033
	extPQOnly    uint16 = 0xFF01
	extPSK       uint16 = 0x0029
	extEarlyData uint16 = 0x002A
)

type messageFrame struct {
	version    uint8
	streamID   uint32
	sequenceNo uint64
	flags      uint8
	payload    []byte
	aad        []byte
}

func (f *messageFrame) marshal() []byte {
	size := 1 + 4 + 8 + 1 + 4 + len(f.payload) + 2 + len(f.aad)
	buf := make([]byte, size)
	offset := 0

	buf[offset] = f.version
	offset++

	binary.BigEndian.PutUint32(buf[offset:], f.streamID)
	offset += 4

	binary.BigEndian.PutUint64(buf[offset:], f.sequenceNo)
	offset += 8

	buf[offset] = f.flags
	offset++

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(f.payload)))
	offset += 4
	copy(buf[offset:], f.payload)
	offset += len(f.payload)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(f.aad)))
	offset += 2
	copy(buf[offset:], f.aad)

	return buf
}

func (f *messageFrame) unmarshal(data []byte) error {
	if len(data) < 18 {
		return errors.New("frame too short")
	}

	offset := 0
	f.version = data[offset]
	offset++

	f.streamID = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	f.sequenceNo = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	f.flags = data[offset]
	offset++

	payloadLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	if len(data) < offset+int(payloadLen)+2 {
		return errors.New("invalid frame length")
	}

	f.payload = make([]byte, payloadLen)
	copy(f.payload, data[offset:])
	offset += int(payloadLen)

	aadLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2

	if len(data) < offset+int(aadLen) {
		return errors.New("invalid AAD length")
	}

	f.aad = make([]byte, aadLen)
	copy(f.aad, data[offset:])

	return nil
}

type keyUpdateMessage struct {
	timestamp int64
	newKeyID  uint64
}

func (m *keyUpdateMessage) marshal() []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:8], uint64(m.timestamp))
	binary.BigEndian.PutUint64(buf[8:16], m.newKeyID)
	return buf
}

type closeMessage struct {
	Reason string
}

func (m *closeMessage) marshal() []byte {
	return []byte(m.Reason)
}
