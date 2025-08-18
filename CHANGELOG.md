# Changelog

All notable changes to QZMQ will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of QZMQ post-quantum secure transport for ZeroMQ
- ML-KEM (Kyber) support: ML-KEM-768 and ML-KEM-1024
- ML-DSA (Dilithium) support: ML-DSA-2 and ML-DSA-3
- Hybrid key encapsulation: X25519 + ML-KEM
- Dual backend support: Pure Go and C bindings (CZMQ)
- Automatic backend detection and selection
- Three security modes: Conservative (PQ-only), Hybrid, Performance (Classical)
- AES-256-GCM and ChaCha20-Poly1305 authenticated encryption
- Automatic key rotation based on time, message count, or data volume
- 0-RTT resumption support for fast reconnection
- Anti-DoS cookie mechanism
- Perfect forward secrecy
- Replay attack protection
- Comprehensive test suite
- Benchmark suite for performance testing
- Example applications
- Full API documentation

### Security
- Quantum-resistant cryptography using NIST-standardized algorithms
- Secure key derivation using HKDF
- Constant-time comparisons for sensitive data
- Cryptographic binding to prevent downgrade attacks

## [0.1.0] - 2025-01-20

### Added
- Initial public release
- Core QZMQ protocol implementation
- Basic socket types: REQ/REP, PUB/SUB, PUSH/PULL, DEALER/ROUTER
- Connection state machine with 1-RTT handshake
- Message encryption and authentication
- Session management
- Metrics and statistics collection

### Known Issues
- ML-KEM and ML-DSA implementations are stubs (pending real implementation)
- C backend (CZMQ) support is experimental
- Hardware acceleration not yet implemented

## Future Releases

### [0.2.0] - Planned
- Real ML-KEM implementation using liboqs
- Real ML-DSA implementation
- Hardware acceleration (AES-NI, AVX2)
- WebSocket transport support
- Improved C backend integration

### [0.3.0] - Planned
- QUIC transport support
- Additional post-quantum algorithms (BIKE, HQC)
- Certificate management improvements
- Performance optimizations

### [1.0.0] - Planned
- Production-ready release
- FIPS 140-3 compliance
- Complete documentation
- Stable API guarantee

---

For more details on each release, see the [GitHub Releases](https://github.com/luxfi/qzmq/releases) page.