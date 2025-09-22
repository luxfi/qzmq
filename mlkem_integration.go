// Copyright (C) 2025, Lux Industries Inc. All rights reserved.
// ML-KEM Integration for QZMQ Post-Quantum Key Exchange

package qzmq

import (
	"crypto/rand"
	"errors"
	
	"github.com/luxfi/crypto/mlkem"
)

// RealMLKEM768 implements ML-KEM-768 using the actual FIPS 203 implementation
type RealMLKEM768 struct{}

func (k *RealMLKEM768) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	// Generate real ML-KEM-768 key pair
	priv, pub, err := mlkem.GenerateKeyPair(rand.Reader, mlkem.MLKEM768)
	if err != nil {
		return nil, nil, err
	}
	
	// Wrap in our interface
	pk := &mlkemPublicKey{key: pub}
	sk := &mlkemPrivateKey{key: priv, public: pk}
	
	return pk, sk, nil
}

func (k *RealMLKEM768) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	mpk, ok := pk.(*mlkemPublicKey)
	if !ok {
		return nil, nil, errors.New("invalid public key type")
	}
	
	// Perform real encapsulation
	result, err := mpk.key.Encapsulate(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	
	// Extract ciphertext and shared secret from result
	return result.Ciphertext, result.SharedSecret, nil
}

func (k *RealMLKEM768) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	msk, ok := sk.(*mlkemPrivateKey)
	if !ok {
		return nil, errors.New("invalid private key type")
	}
	
	// Perform real decapsulation
	sharedSecret, err := msk.key.Decapsulate(ciphertext)
	if err != nil {
		return nil, err
	}
	
	return sharedSecret, nil
}

func (k *RealMLKEM768) PublicKeySize() int    { return mlkem.MLKEM768PublicKeySize }
func (k *RealMLKEM768) PrivateKeySize() int   { return mlkem.MLKEM768PrivateKeySize }
func (k *RealMLKEM768) CiphertextSize() int   { return mlkem.MLKEM768CiphertextSize }
func (k *RealMLKEM768) SharedSecretSize() int { return mlkem.MLKEM768SharedSecretSize }

// RealMLKEM1024 implements ML-KEM-1024 using the actual FIPS 203 implementation
type RealMLKEM1024 struct{}

func (k *RealMLKEM1024) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	// Generate real ML-KEM-1024 key pair
	priv, pub, err := mlkem.GenerateKeyPair(rand.Reader, mlkem.MLKEM1024)
	if err != nil {
		return nil, nil, err
	}
	
	// Wrap in our interface
	pk := &mlkemPublicKey{key: pub}
	sk := &mlkemPrivateKey{key: priv, public: pk}
	
	return pk, sk, nil
}

func (k *RealMLKEM1024) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	mpk, ok := pk.(*mlkemPublicKey)
	if !ok {
		return nil, nil, errors.New("invalid public key type")
	}
	
	// Perform real encapsulation
	result, err := mpk.key.Encapsulate(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	
	// Extract ciphertext and shared secret from result
	return result.Ciphertext, result.SharedSecret, nil
}

func (k *RealMLKEM1024) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	msk, ok := sk.(*mlkemPrivateKey)
	if !ok {
		return nil, errors.New("invalid private key type")
	}
	
	// Perform real decapsulation
	sharedSecret, err := msk.key.Decapsulate(ciphertext)
	if err != nil {
		return nil, err
	}
	
	return sharedSecret, nil
}

func (k *RealMLKEM1024) PublicKeySize() int    { return mlkem.MLKEM1024PublicKeySize }
func (k *RealMLKEM1024) PrivateKeySize() int   { return mlkem.MLKEM1024PrivateKeySize }
func (k *RealMLKEM1024) CiphertextSize() int   { return mlkem.MLKEM1024CiphertextSize }
func (k *RealMLKEM1024) SharedSecretSize() int { return mlkem.MLKEM1024SharedSecretSize }

// mlkemPublicKey wraps an ML-KEM public key
type mlkemPublicKey struct {
	key *mlkem.PublicKey
}

func (pk *mlkemPublicKey) Bytes() []byte {
	return pk.key.Bytes()
}

// mlkemPrivateKey wraps an ML-KEM private key
type mlkemPrivateKey struct {
	key    *mlkem.PrivateKey
	public PublicKey
}

func (sk *mlkemPrivateKey) Bytes() []byte {
	return sk.key.Bytes()
}

func (sk *mlkemPrivateKey) Public() PublicKey {
	return sk.public
}

// HybridMLKEM implements hybrid X25519 + ML-KEM-768
type HybridMLKEM struct {
	classical *X25519KEM
	pq        *RealMLKEM768
}

func NewHybridMLKEM() *HybridMLKEM {
	return &HybridMLKEM{
		classical: &X25519KEM{},
		pq:        &RealMLKEM768{},
	}
}

func (h *HybridMLKEM) GenerateKeyPair() (PublicKey, PrivateKey, error) {
	// Generate both key pairs
	classicalPub, classicalPriv, err := h.classical.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	
	pqPub, pqPriv, err := h.pq.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	
	// Combine keys
	pk := &hybridPublicKey{
		classical: classicalPub,
		pq:        pqPub,
	}
	
	sk := &hybridPrivateKey{
		classical: classicalPriv,
		pq:        pqPriv,
		public:    pk,
	}
	
	return pk, sk, nil
}

func (h *HybridMLKEM) Encapsulate(pk PublicKey) ([]byte, []byte, error) {
	hpk, ok := pk.(*hybridPublicKey)
	if !ok {
		return nil, nil, errors.New("invalid public key type")
	}
	
	// Encapsulate with both algorithms
	classicalCt, classicalSS, err := h.classical.Encapsulate(hpk.classical)
	if err != nil {
		return nil, nil, err
	}
	
	pqCt, pqSS, err := h.pq.Encapsulate(hpk.pq)
	if err != nil {
		return nil, nil, err
	}
	
	// Combine ciphertexts and shared secrets
	ciphertext := append(classicalCt, pqCt...)
	
	// XOR the shared secrets for hybrid security
	sharedSecret := make([]byte, 32)
	for i := 0; i < 32; i++ {
		sharedSecret[i] = classicalSS[i] ^ pqSS[i]
	}
	
	return ciphertext, sharedSecret, nil
}

func (h *HybridMLKEM) Decapsulate(sk PrivateKey, ciphertext []byte) ([]byte, error) {
	hsk, ok := sk.(*hybridPrivateKey)
	if !ok {
		return nil, errors.New("invalid private key type")
	}
	
	// Split ciphertext
	classicalCtSize := h.classical.CiphertextSize()
	if len(ciphertext) < classicalCtSize {
		return nil, errors.New("ciphertext too short")
	}
	
	classicalCt := ciphertext[:classicalCtSize]
	pqCt := ciphertext[classicalCtSize:]
	
	// Decapsulate with both algorithms
	classicalSS, err := h.classical.Decapsulate(hsk.classical, classicalCt)
	if err != nil {
		return nil, err
	}
	
	pqSS, err := h.pq.Decapsulate(hsk.pq, pqCt)
	if err != nil {
		return nil, err
	}
	
	// XOR the shared secrets
	sharedSecret := make([]byte, 32)
	for i := 0; i < 32; i++ {
		sharedSecret[i] = classicalSS[i] ^ pqSS[i]
	}
	
	return sharedSecret, nil
}

func (h *HybridMLKEM) PublicKeySize() int {
	return h.classical.PublicKeySize() + h.pq.PublicKeySize()
}

func (h *HybridMLKEM) PrivateKeySize() int {
	return h.classical.PrivateKeySize() + h.pq.PrivateKeySize()
}

func (h *HybridMLKEM) CiphertextSize() int {
	return h.classical.CiphertextSize() + h.pq.CiphertextSize()
}

func (h *HybridMLKEM) SharedSecretSize() int {
	return 32 // Always 32 bytes after XOR
}

// hybridPublicKey represents a hybrid public key
type hybridPublicKey struct {
	classical PublicKey
	pq        PublicKey
}

func (pk *hybridPublicKey) Bytes() []byte {
	return append(pk.classical.Bytes(), pk.pq.Bytes()...)
}

// hybridPrivateKey represents a hybrid private key
type hybridPrivateKey struct {
	classical PrivateKey
	pq        PrivateKey
	public    PublicKey
}

func (sk *hybridPrivateKey) Bytes() []byte {
	return append(sk.classical.Bytes(), sk.pq.Bytes()...)
}

func (sk *hybridPrivateKey) Public() PublicKey {
	return sk.public
}

// UpdateKEMFactory updates the getKEM function to use real implementations
func UpdateKEMFactory() {
	// This would replace the stub implementations with real ones
	// in the actual getKEM function
}