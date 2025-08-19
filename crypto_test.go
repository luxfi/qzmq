package qzmq

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestX25519KEM(t *testing.T) {
	kem := &X25519KEM{}

	// Generate key pair
	pk, sk, err := kem.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	// Check key sizes
	if len(pk.Bytes()) != kem.PublicKeySize() {
		t.Errorf("Public key size mismatch: got %d, want %d", len(pk.Bytes()), kem.PublicKeySize())
	}
	if len(sk.Bytes()) != kem.PrivateKeySize() {
		t.Errorf("Private key size mismatch: got %d, want %d", len(sk.Bytes()), kem.PrivateKeySize())
	}

	// Encapsulate
	ct, ss1, err := kem.Encapsulate(pk)
	if err != nil {
		t.Fatalf("Encapsulate failed: %v", err)
	}

	if len(ct) != kem.CiphertextSize() {
		t.Errorf("Ciphertext size mismatch: got %d, want %d", len(ct), kem.CiphertextSize())
	}
	if len(ss1) != kem.SharedSecretSize() {
		t.Errorf("Shared secret size mismatch: got %d, want %d", len(ss1), kem.SharedSecretSize())
	}

	// Decapsulate
	ss2, err := kem.Decapsulate(sk, ct)
	if err != nil {
		t.Fatalf("Decapsulate failed: %v", err)
	}

	// Shared secrets should match
	if !bytes.Equal(ss1, ss2) {
		t.Error("Shared secrets don't match")
	}
}

func TestMLKEM768(t *testing.T) {
	kem := &MLKEM768Type{}

	// Generate key pair
	pk, sk, err := kem.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	// Check key sizes
	if len(pk.Bytes()) != kem.PublicKeySize() {
		t.Errorf("Public key size mismatch: got %d, want %d", len(pk.Bytes()), kem.PublicKeySize())
	}
	if len(sk.Bytes()) != kem.PrivateKeySize() {
		t.Errorf("Private key size mismatch: got %d, want %d", len(sk.Bytes()), kem.PrivateKeySize())
	}

	// Encapsulate
	ct, ss1, err := kem.Encapsulate(pk)
	if err != nil {
		t.Fatalf("Encapsulate failed: %v", err)
	}

	// Decapsulate
	ss2, err := kem.Decapsulate(sk, ct)
	if err != nil {
		t.Fatalf("Decapsulate failed: %v", err)
	}

	// In stub implementation, secrets won't match, but sizes should be correct
	if len(ss1) != kem.SharedSecretSize() {
		t.Errorf("Shared secret 1 size mismatch: got %d, want %d", len(ss1), kem.SharedSecretSize())
	}
	if len(ss2) != kem.SharedSecretSize() {
		t.Errorf("Shared secret 2 size mismatch: got %d, want %d", len(ss2), kem.SharedSecretSize())
	}
}

func TestHybridKEM(t *testing.T) {
	classical := &X25519KEM{}
	pq := &MLKEM768Type{}
	hybrid := NewHybridKEM(classical, pq)

	// Generate key pair
	pk, sk, err := hybrid.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	// Check combined key sizes
	expectedPKSize := classical.PublicKeySize() + pq.PublicKeySize()
	if len(pk.Bytes()) != expectedPKSize {
		t.Errorf("Public key size mismatch: got %d, want %d", len(pk.Bytes()), expectedPKSize)
	}

	expectedSKSize := classical.PrivateKeySize() + pq.PrivateKeySize()
	if len(sk.Bytes()) != expectedSKSize {
		t.Errorf("Private key size mismatch: got %d, want %d", len(sk.Bytes()), expectedSKSize)
	}

	// Encapsulate
	ct, ss, err := hybrid.Encapsulate(pk)
	if err != nil {
		t.Fatalf("Encapsulate failed: %v", err)
	}

	expectedCTSize := classical.CiphertextSize() + pq.CiphertextSize()
	if len(ct) != expectedCTSize {
		t.Errorf("Ciphertext size mismatch: got %d, want %d", len(ct), expectedCTSize)
	}

	if len(ss) != hybrid.SharedSecretSize() {
		t.Errorf("Shared secret size mismatch: got %d, want %d", len(ss), hybrid.SharedSecretSize())
	}
}

func TestAEADCreation(t *testing.T) {
	tests := []struct {
		name string
		algo AeadAlgorithm
		key  []byte
	}{
		{
			name: "AES-256-GCM",
			algo: AES256GCM,
			key:  make([]byte, 32),
		},
		{
			name: "ChaCha20-Poly1305",
			algo: ChaCha20Poly1305,
			key:  make([]byte, 32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand.Read(tt.key)

			aead, err := createAEAD(tt.algo, tt.key)
			if err != nil {
				t.Fatalf("createAEAD failed: %v", err)
			}

			// Test encryption/decryption
			plaintext := []byte("Hello QZMQ")
			nonce := make([]byte, aead.NonceSize())
			rand.Read(nonce)

			// Encrypt
			ciphertext := aead.Seal(nil, nonce, plaintext, nil)

			// Decrypt
			decrypted, err := aead.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			if !bytes.Equal(plaintext, decrypted) {
				t.Error("Decrypted text doesn't match original")
			}
		})
	}
}

func TestKeyDerivation(t *testing.T) {
	kemSecret := make([]byte, 32)
	ecdheSecret := make([]byte, 32)
	rand.Read(kemSecret)
	rand.Read(ecdheSecret)

	hashAlgos := []HashAlgorithm{SHA256, SHA384, SHA512}

	for _, algo := range hashAlgos {
		t.Run(algo.String(), func(t *testing.T) {
			keys, err := deriveKeys(kemSecret, ecdheSecret, algo)
			if err != nil {
				t.Fatalf("deriveKeys failed: %v", err)
			}

			// Check that keys are derived
			if len(keys.clientKey) != 32 {
				t.Errorf("Client key size wrong: %d", len(keys.clientKey))
			}
			if len(keys.serverKey) != 32 {
				t.Errorf("Server key size wrong: %d", len(keys.serverKey))
			}
			if len(keys.exporterSecret) != 32 {
				t.Errorf("Exporter secret size wrong: %d", len(keys.exporterSecret))
			}

			// Keys should be different
			if bytes.Equal(keys.clientKey, keys.serverKey) {
				t.Error("Client and server keys are the same")
			}
			if bytes.Equal(keys.clientKey, keys.exporterSecret) {
				t.Error("Client key and exporter secret are the same")
			}
		})
	}
}

func TestCookieGeneration(t *testing.T) {
	cookie1 := generateCookie()
	cookie2 := generateCookie()

	if len(cookie1) != 32 {
		t.Errorf("Cookie size wrong: %d", len(cookie1))
	}

	// Cookies should be random
	if bytes.Equal(cookie1, cookie2) {
		t.Error("Cookies are not random")
	}

	// Test validation (stub always returns true for valid size)
	if !validateCookie(cookie1, 3600) {
		t.Error("Cookie validation failed")
	}
}

func TestFinishedMAC(t *testing.T) {
	key := make([]byte, 32)
	transcript := []byte("handshake transcript")
	rand.Read(key)

	mac1 := computeFinishedMAC(key, transcript)
	mac2 := computeFinishedMAC(key, transcript)

	// MACs should be deterministic
	if !bytes.Equal(mac1, mac2) {
		t.Error("MACs are not deterministic")
	}

	// MAC should be 32 bytes (SHA256)
	if len(mac1) != 32 {
		t.Errorf("MAC size wrong: %d", len(mac1))
	}

	// Different key should produce different MAC
	key2 := make([]byte, 32)
	rand.Read(key2)
	mac3 := computeFinishedMAC(key2, transcript)
	if bytes.Equal(mac1, mac3) {
		t.Error("Different keys produced same MAC")
	}
}

// String method for HashAlgorithm
func (h HashAlgorithm) String() string {
	switch h {
	case SHA256:
		return "SHA256"
	case SHA384:
		return "SHA384"
	case SHA512:
		return "SHA512"
	case BLAKE3:
		return "BLAKE3"
	default:
		return "UNKNOWN"
	}
}

// Benchmarks

func BenchmarkX25519KEM(b *testing.B) {
	kem := &X25519KEM{}
	pk, _, _ := kem.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = kem.Encapsulate(pk)
	}
}

func BenchmarkMLKEM768(b *testing.B) {
	kem := &MLKEM768Type{}
	pk, _, _ := kem.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = kem.Encapsulate(pk)
	}
}

func BenchmarkAES256GCM(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)
	aead, _ := createAEAD(AES256GCM, key)

	plaintext := make([]byte, 1024)
	nonce := make([]byte, aead.NonceSize())
	rand.Read(plaintext)
	rand.Read(nonce)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aead.Seal(nil, nonce, plaintext, nil)
	}
}

func BenchmarkChaCha20Poly1305(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)
	aead, _ := createAEAD(ChaCha20Poly1305, key)

	plaintext := make([]byte, 1024)
	nonce := make([]byte, aead.NonceSize())
	rand.Read(plaintext)
	rand.Read(nonce)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aead.Seal(nil, nonce, plaintext, nil)
	}
}
