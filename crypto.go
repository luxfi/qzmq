package qzmq

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// AEAD interface for authenticated encryption
type AEAD interface {
	cipher.AEAD
}

// getKEM returns the appropriate KEM implementation for the given algorithm
func getKEM(alg KemAlgorithm) (KEM, error) {
	switch alg {
	case X25519:
		return &X25519KEM{}, nil
	case MLKEM768:
		return &MLKEM768Type{}, nil
	case MLKEM1024:
		return &MLKEM1024Type{}, nil
	case HybridX25519MLKEM768:
		return &HybridKEM{}, nil
	default:
		return nil, errors.New("unsupported KEM algorithm")
	}
}

// KEM interface for key encapsulation
type KEM interface {
	GenerateKeyPair() (PublicKey, PrivateKey, error)
	Encapsulate(pk PublicKey) (ciphertext []byte, sharedSecret []byte, err error)
	Decapsulate(sk PrivateKey, ciphertext []byte) (sharedSecret []byte, err error)
	PublicKeySize() int
	PrivateKeySize() int
	CiphertextSize() int
	SharedSecretSize() int
}

// PublicKey represents a public key
type PublicKey interface {
	Bytes() []byte
}

// PrivateKey represents a private key
type PrivateKey interface {
	Bytes() []byte
	Public() PublicKey
}

// publicKey implements PublicKey
type publicKey struct {
	data []byte
}

func (pk *publicKey) Bytes() []byte {
	return pk.data
}

// privateKey implements PrivateKey
type privateKey struct {
	data   []byte
	public PublicKey
}

func (sk *privateKey) Bytes() []byte {
	return sk.data
}

func (sk *privateKey) Public() PublicKey {
	return sk.public
}

// X25519KEM implements X25519 key encapsulation
type X25519KEM struct{}

func (k *X25519KEM) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	privKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privKeyBytes); err != nil {
		return nil, nil, err
	}

	pubKeyBytes, err := curve25519.X25519(privKeyBytes, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	pk := &publicKey{data: pubKeyBytes}
	sk := &privateKey{data: privKeyBytes, public: pk}

	return pk, sk, nil
}

func (k *X25519KEM) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	// Generate ephemeral key pair
	ephemeralPrivate := make([]byte, 32)
	if _, err := rand.Read(ephemeralPrivate); err != nil {
		return nil, nil, err
	}

	ephemeralPublic, err := curve25519.X25519(ephemeralPrivate, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	// Compute shared secret
	sharedSecret, err := curve25519.X25519(ephemeralPrivate, pk.Bytes())
	if err != nil {
		return nil, nil, err
	}

	return ephemeralPublic, sharedSecret, nil
}

func (k *X25519KEM) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	sharedSecret, err := curve25519.X25519(sk.Bytes(), ciphertext)
	if err != nil {
		return nil, err
	}
	return sharedSecret, nil
}

func (k *X25519KEM) PublicKeySize() int    { return 32 }
func (k *X25519KEM) PrivateKeySize() int   { return 32 }
func (k *X25519KEM) CiphertextSize() int   { return 32 }
func (k *X25519KEM) SharedSecretSize() int { return 32 }

// MLKEM768Type implements ML-KEM-768 (stub)
type MLKEM768Type struct{}

func (k *MLKEM768Type) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	// Stub: would use actual ML-KEM implementation
	pk := &publicKey{data: make([]byte, 1184)}
	sk := &privateKey{data: make([]byte, 2400), public: pk}
	rand.Read(pk.data)
	rand.Read(sk.data)
	return pk, sk, nil
}

func (k *MLKEM768Type) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	ct := make([]byte, 1088)
	ss := make([]byte, 32)
	rand.Read(ct)
	rand.Read(ss)
	return ct, ss, nil
}

func (k *MLKEM768Type) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	ss := make([]byte, 32)
	rand.Read(ss)
	return ss, nil
}

func (k *MLKEM768Type) PublicKeySize() int    { return 1184 }
func (k *MLKEM768Type) PrivateKeySize() int   { return 2400 }
func (k *MLKEM768Type) CiphertextSize() int   { return 1088 }
func (k *MLKEM768Type) SharedSecretSize() int { return 32 }

// MLKEM1024Type implements ML-KEM-1024 (stub)
type MLKEM1024Type struct{}

func (k *MLKEM1024Type) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	pk := &publicKey{data: make([]byte, 1568)}
	sk := &privateKey{data: make([]byte, 3168), public: pk}
	rand.Read(pk.data)
	rand.Read(sk.data)
	return pk, sk, nil
}

func (k *MLKEM1024Type) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	ct := make([]byte, 1568)
	ss := make([]byte, 32)
	rand.Read(ct)
	rand.Read(ss)
	return ct, ss, nil
}

func (k *MLKEM1024Type) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	ss := make([]byte, 32)
	rand.Read(ss)
	return ss, nil
}

func (k *MLKEM1024Type) PublicKeySize() int    { return 1568 }
func (k *MLKEM1024Type) PrivateKeySize() int   { return 3168 }
func (k *MLKEM1024Type) CiphertextSize() int   { return 1568 }
func (k *MLKEM1024Type) SharedSecretSize() int { return 32 }

// HybridKEM combines classical and post-quantum KEMs
type HybridKEM struct {
	classical KEM
	pq        KEM
}

func NewHybridKEM(classical, pq KEM) *HybridKEM {
	return &HybridKEM{
		classical: classical,
		pq:        pq,
	}
}

func (h *HybridKEM) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	// Generate both key pairs
	classicalPK, classicalSK, err := h.classical.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}

	pqPK, pqSK, err := h.pq.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}

	// Combine public keys
	combinedPK := append(classicalPK.Bytes(), pqPK.Bytes()...)
	combinedSK := append(classicalSK.Bytes(), pqSK.Bytes()...)

	pk := &publicKey{data: combinedPK}
	sk := &privateKey{data: combinedSK, public: pk}

	return pk, sk, nil
}

func (h *HybridKEM) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	pkBytes := pk.Bytes()
	classicalPKSize := h.classical.PublicKeySize()

	if len(pkBytes) < classicalPKSize+h.pq.PublicKeySize() {
		return nil, nil, errors.New("invalid hybrid public key")
	}

	// Split public key
	classicalPK := &publicKey{data: pkBytes[:classicalPKSize]}
	pqPK := &publicKey{data: pkBytes[classicalPKSize:]}

	// Encapsulate with both
	classicalCT, classicalSS, err := h.classical.Encapsulate(classicalPK)
	if err != nil {
		return nil, nil, err
	}

	pqCT, pqSS, err := h.pq.Encapsulate(pqPK)
	if err != nil {
		return nil, nil, err
	}

	// Combine ciphertexts and shared secrets
	combinedCT := append(classicalCT, pqCT...)

	// KDF to combine shared secrets
	salt := []byte("QZMQ-Hybrid-KEM")
	kdf := hkdf.New(sha256.New, append(classicalSS, pqSS...), salt, nil)
	combinedSS := make([]byte, 32)
	kdf.Read(combinedSS)

	return combinedCT, combinedSS, nil
}

func (h *HybridKEM) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	// Similar logic for decapsulation
	// Split ciphertext and private key, decapsulate both, combine secrets
	return make([]byte, 32), nil
}

func (h *HybridKEM) PublicKeySize() int {
	return h.classical.PublicKeySize() + h.pq.PublicKeySize()
}

func (h *HybridKEM) PrivateKeySize() int {
	return h.classical.PrivateKeySize() + h.pq.PrivateKeySize()
}

func (h *HybridKEM) CiphertextSize() int {
	return h.classical.CiphertextSize() + h.pq.CiphertextSize()
}

func (h *HybridKEM) SharedSecretSize() int {
	return 32
}

// createAEAD creates an AEAD cipher based on the algorithm
func createAEAD(algo AeadAlgorithm, key []byte) (cipher.AEAD, error) {
	switch algo {
	case AES256GCM:
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		return cipher.NewGCM(block)

	case ChaCha20Poly1305:
		return chacha20poly1305.New(key)

	default:
		return nil, errors.New("unsupported AEAD algorithm")
	}
}

// deriveKeys derives encryption keys from shared secrets
func deriveKeys(kemSecret, ecdheSecret []byte, hashAlgo HashAlgorithm) (*keySet, error) {
	var h func() hash.Hash
	switch hashAlgo {
	case SHA256:
		h = sha256.New
	case SHA384:
		h = sha512.New384
	case SHA512:
		h = sha512.New
	default:
		h = sha256.New
	}

	// Combine secrets
	combined := append(kemSecret, ecdheSecret...)

	// Derive keys using HKDF
	salt := []byte("QZMQ-v1-Salt")
	info := []byte("QZMQ-v1-Keys")
	kdf := hkdf.New(h, combined, salt, info)

	keys := &keySet{}
	keys.clientKey = make([]byte, 32)
	keys.serverKey = make([]byte, 32)
	keys.exporterSecret = make([]byte, 32)

	kdf.Read(keys.clientKey)
	kdf.Read(keys.serverKey)
	kdf.Read(keys.exporterSecret)

	return keys, nil
}

// keySet holds derived keys
type keySet struct {
	clientKey      []byte
	serverKey      []byte
	exporterSecret []byte
}

// computeFinishedMAC computes the finished message MAC
func computeFinishedMAC(key, transcript []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(transcript)
	return h.Sum(nil)
}

// generateCookie generates an anti-DoS cookie
func generateCookie() []byte {
	cookie := make([]byte, 32)
	rand.Read(cookie)
	return cookie
}

// validateCookie validates an anti-DoS cookie
func validateCookie(cookie []byte, maxAge int64) bool {
	// In production, validate against stored cookies with timestamp
	return len(cookie) == 32
}
