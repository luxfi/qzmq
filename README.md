# QZMQ - Quantum-Safe ZeroMQ

[![Go Reference](https://pkg.go.dev/badge/github.com/luxfi/qzmq.svg)](https://pkg.go.dev/github.com/luxfi/qzmq)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

QZMQ (QuantumZMQ) is a post-quantum secure transport layer for ZeroMQ that provides quantum-resistant encryption, authentication, and key exchange using NIST-standardized algorithms.

## Features

- 🔒 **Post-Quantum Security**: ML-KEM (Kyber) and ML-DSA (Dilithium) algorithms
- 🚀 **High Performance**: Hardware-accelerated AES-GCM, zero-copy operations
- 🔑 **Hybrid Modes**: Combine classical (X25519) and post-quantum algorithms
- ⚡ **0-RTT Resumption**: Fast reconnection with session tickets
- 🛡️ **DoS Protection**: Stateless cookies and rate limiting
- 🔄 **Automatic Key Rotation**: Time and volume-based key updates

## Installation

```bash
go get github.com/luxfi/qzmq
```

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "github.com/luxfi/qzmq"
)

func main() {
    // Create QZMQ transport with default options
    transport, err := qzmq.New(qzmq.DefaultOptions())
    if err != nil {
        log.Fatal(err)
    }
    defer transport.Close()

    // Server
    server, err := transport.NewSocket(qzmq.REP)
    if err != nil {
        log.Fatal(err)
    }
    server.Bind("tcp://*:5555")

    // Client
    client, err := transport.NewSocket(qzmq.REQ)
    if err != nil {
        log.Fatal(err)
    }
    client.Connect("tcp://localhost:5555")

    // Send quantum-safe encrypted message
    client.Send([]byte("Hello Quantum World"))
    
    // Receive and decrypt
    msg, _ := server.Recv()
    log.Printf("Received: %s", msg)
}
```

### Advanced Configuration

```go
// Configure for maximum quantum security
opts := qzmq.Options{
    Suite: qzmq.Suite{
        KEM:  qzmq.MLKEM1024,    // Strongest post-quantum KEM
        Sign: qzmq.MLDSA3,       // Strongest signatures
        AEAD: qzmq.AES256GCM,    // Hardware-accelerated
    },
    Mode: qzmq.ModePQOnly,       // Reject non-PQ algorithms
    KeyRotation: qzmq.KeyRotationPolicy{
        MaxMessages: 1<<30,      // Rotate after 1B messages
        MaxBytes:    1<<40,      // Rotate after 1TB
        MaxAge:      5*time.Minute,
    },
}

transport, err := qzmq.New(opts)
```

## Cryptographic Suites

| Suite | KEM | Signature | AEAD | Security Level |
|-------|-----|-----------|------|----------------|
| **Conservative** | ML-KEM-1024 | ML-DSA-3 | AES-256-GCM | 192-bit |
| **Balanced** | ML-KEM-768 | ML-DSA-2 | AES-256-GCM | 128-bit |
| **Hybrid** | X25519+ML-KEM-768 | ML-DSA-2 | AES-256-GCM | 128-bit |
| **Performance** | X25519 | Ed25519 | ChaCha20-Poly1305 | 128-bit |

## Performance

Benchmark results on Apple M2:

```
BenchmarkHandshake/Classical-8         5000    235124 ns/op
BenchmarkHandshake/Hybrid-8            2000    892341 ns/op
BenchmarkHandshake/PQOnly-8            1000   1243567 ns/op
BenchmarkEncrypt/AES256GCM-8        1000000      1053 ns/op
BenchmarkEncrypt/ChaCha20-8          500000      2341 ns/op
```

Throughput:
- Classical: 10 Gbps
- Hybrid: 8 Gbps  
- PQ-Only: 6 Gbps

## Security Properties

✅ **Quantum Resistance**: Secure against attacks by quantum computers  
✅ **Perfect Forward Secrecy**: Past sessions remain secure if keys are compromised  
✅ **Authenticated Encryption**: All messages are encrypted and authenticated  
✅ **Replay Protection**: Sequence numbers prevent message replay  
✅ **Downgrade Protection**: Cryptographic binding prevents algorithm downgrade  

## Architecture

```
QZMQ Transport Layer
├── Backend Abstraction
│   └── Go Backend (pebbe/zmq4)
├── Cryptographic Core
│   ├── KEM (ML-KEM, X25519, Hybrid)
│   ├── Signatures (ML-DSA, Ed25519)
│   ├── AEAD (AES-GCM, ChaCha20)
│   └── KDF (HKDF-SHA256/384)
├── Protocol Engine
│   ├── Handshake (1-RTT)
│   ├── Session Management
│   ├── Key Rotation
│   └── 0-RTT Resumption
└── Security Features
    ├── Anti-DoS Cookies
    ├── Rate Limiting
    └── Certificate Validation
```

## Examples

See the [examples/](examples/) directory for:
- [Basic client/server](examples/basic/)
- [Pub/sub with encryption](examples/pubsub/)
- [Router/dealer patterns](examples/router/)
- [Key rotation demo](examples/rotation/)
- [Performance testing](examples/benchmark/)

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Migration from CurveZMQ

QZMQ is designed as a drop-in replacement for CurveZMQ with quantum security.

## Roadmap

- [x] Core implementation
- [x] Pure Go backend
- [x] C bindings support
- [ ] Hardware acceleration (AES-NI, AVX2)
- [ ] FIPS 140-3 certification
- [ ] WebSocket transport
- [ ] QUIC transport

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## References

- [NIST Post-Quantum Cryptography](https://csrc.nist.gov/projects/post-quantum-cryptography)
- [ML-KEM (FIPS 203)](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.203.pdf)
- [ML-DSA (FIPS 204)](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.204.pdf)
- [ZeroMQ Security (ZMTP)](https://rfc.zeromq.org/spec/26/)

## Support

- 📧 Email: security@lux.network
- 💬 Discord: [Lux Network](https://discord.gg/lux)
- 🐛 Issues: [GitHub Issues](https://github.com/luxfi/qzmq/issues)

---

Built with ❤️ by [Lux Network](https://lux.network) for a quantum-safe future.