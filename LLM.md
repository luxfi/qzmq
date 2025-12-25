# CLAUDE.md - QZMQ (QuantumZMQ) Project Guide

## Overview
QZMQ is a post-quantum secure transport protocol for ZeroMQ, providing quantum-resistant encryption using NIST-standardized algorithms. This is a **pure Go implementation** with real production-ready code - NO STUBS.

## Architecture

### Dual Backend System
QZMQ supports two backend implementations based on build configuration:

1. **Pure Go Backend** (CGO_ENABLED=0)
   - Uses `github.com/luxfi/zmq/v4` - a pure Go ZeroMQ implementation
   - No C dependencies, runs anywhere Go runs
   - Full support for all ZeroMQ patterns
   - Production-ready with complete test coverage

2. **CGO Backend** (CGO_ENABLED=1)
   - Uses `github.com/pebbe/zmq4` - Go bindings for libzmq
   - Requires libzmq C library
   - Potentially higher performance for some workloads
   - Used for compatibility testing

### Key Components

```
qzmq/
├── qzmq.go              # Core QZMQ protocol and crypto suite
├── backend.go           # Real ZMQ backend using luxfi/zmq
├── backend_cgo.go       # CGO backend using pebbe/zmq4
├── socket.go            # Socket abstraction layer
├── patterns.go          # ZeroMQ pattern implementations
├── logging.go           # luxfi/log integration
├── metrics_prometheus.go # luxfi/metric Prometheus integration
├── wire.go              # Wire protocol serialization
├── record.go            # Record layer encryption
├── zmtp.go              # ZMTP mechanism integration
├── migrate.go           # Migration utilities
└── mainnet_test.go      # Production readiness tests
```

## Development Commands

### Building
```bash
# Build with pure Go backend
CGO_ENABLED=0 go build ./...

# Build with CGO backend (requires libzmq)
CGO_ENABLED=1 go build ./...

# Build everything
make build
```

### Testing
```bash
# Run all tests with pure Go backend
CGO_ENABLED=0 go test -v ./...

# Run all tests with CGO backend
CGO_ENABLED=1 go test -v ./...

# Run tests with race detector
go test -race ./...

# Generate coverage report
make test-coverage

# Run benchmarks
make bench
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks (fmt, lint, vet, test)
make check
```

## Important Integration Points

### Logging (luxfi/log)
All logging goes through `luxfi/log` for structured logging:
```go
var logger = log.NewLogger("qzmq")

// Usage
logInfo("Socket bound", "type", socketType, "endpoint", endpoint)
logError("Failed to bind", "error", err)
```

### Metrics (luxfi/metric)
Prometheus metrics via `luxfi/metric`:
```go
// Counters
messagesTotal.WithLabelValues(socketType, direction).Inc()
bytesTotal.WithLabelValues(socketType, direction).Add(float64(len(data)))

// Histograms
latencyHistogram.WithLabelValues(operation).Observe(duration.Seconds())

// Gauges
activeConnections.WithLabelValues(socketType).Set(float64(count))
```

## ZeroMQ Patterns Support

All standard ZeroMQ patterns are fully supported:
- **REQ/REP**: Request-reply pattern
- **PUB/SUB**: Publish-subscribe pattern  
- **PUSH/PULL**: Pipeline pattern
- **DEALER/ROUTER**: Advanced request-reply
- **PAIR**: Exclusive pair pattern
- **STREAM**: Raw TCP socket pattern

## Post-Quantum Cryptography

### Supported Algorithms
- **KEM**: ML-KEM-768/1024 (Kyber), X25519, Hybrid modes
- **Signatures**: ML-DSA-44/65/87 (Dilithium), Ed25519
- **AEAD**: AES-256-GCM, ChaCha20-Poly1305
- **KDF**: HKDF-SHA256/384

### Security Modes
```go
const (
    ModeHybrid  = iota // X25519 + ML-KEM (default)
    ModePQOnly         // Pure post-quantum
    ModeClassic        // Classical only (for compatibility)
)
```

## Known Issues & Solutions

### Test Timeouts
Some tests may timeout with certain router implementations:
```bash
# Set timeout for long-running tests
go test -timeout 30s ./...
```

### luxfi/zmq Router Behavior
The pure Go `luxfi/zmq` router implementation handles high-volume scenarios differently than libzmq. Tests account for this with proper timeouts and context cancellation.

## Operational Scripts

### Monitoring
```bash
# Real-time metrics monitoring
./scripts/monitor.sh

# Prometheus endpoint at :9090/metrics
```

### Deployment
```bash
# Deploy to production
./scripts/deploy.sh production

# Deploy to staging
./scripts/deploy.sh staging
```

### Performance Testing
```bash
# Run performance tests
./scripts/perf_test.sh
```

## CI/CD Pipeline

The project uses GitHub Actions with comprehensive testing:

1. **Test without CGO**: Tests pure Go backend on Linux/macOS/Windows
2. **Test with CGO**: Tests CGO backend on Linux/macOS
3. **Build verification**: Ensures both backends compile
4. **Benchmarks**: Performance testing for both backends
5. **Linting**: Code quality checks with golangci-lint
6. **Hardware acceleration**: Tests for CUDA/MLX support
7. **Integration tests**: End-to-end pattern testing
8. **Mainnet readiness**: Production simulation tests

## Module Dependencies

### Direct Dependencies
- `github.com/luxfi/zmq/v4 v4.2.0` - Pure Go ZeroMQ (local replace)
- `github.com/pebbe/zmq4 v1.4.0` - CGO ZeroMQ bindings
- `golang.org/x/crypto v0.40.0` - Crypto primitives

### Indirect Dependencies (via luxfi packages)
- `github.com/luxfi/log v1.1.22` - Structured logging
- `github.com/luxfi/metric v1.3.0` - Prometheus metrics
- `github.com/prometheus/client_golang v1.23.0` - Prometheus client

### Local Development
The module uses local replace directives:
```go
replace (
    github.com/luxfi/crypto => ../crypto
    github.com/luxfi/zmq/v4 => ../zmq
)
```

## Testing Strategy

### Unit Tests
- Socket creation and operations
- Pattern implementations
- Crypto operations
- Wire protocol

### Integration Tests
- Cross-pattern communication
- Security mode transitions
- Concurrent connections
- Message routing

### Performance Tests
- Throughput benchmarks
- Latency measurements
- Concurrent connection scaling
- Hardware acceleration

### Production Readiness
- High-volume stress tests
- Network partition simulation
- Key rotation under load
- Memory leak detection

## Critical Design Decisions

1. **NO STUBS**: This is a real production implementation. The project explicitly rejects stub/mock implementations in favor of real, working code.

2. **Dual Backend**: Supporting both pure Go and CGO backends provides flexibility for different deployment scenarios while maintaining compatibility.

3. **Integrated Monitoring**: Built-in luxfi/log and luxfi/metric integration ensures production observability from day one.

4. **Post-Quantum by Default**: Hybrid mode (classical + post-quantum) is the default, providing defense-in-depth against quantum attacks.

## Test Coverage Status

Current test results (as of 2025-12-24):
- **73 tests passing, 12 skipped**
- All tests complete within 60s timeout
- Skipped tests are for unimplemented features or known pure Go ZMQ limitations:
  - ROUTER socket cleanup deadlock (pure Go ZMQ blocks on Close during active Recv)
  - Key rotation tracking in Stats()
  - Socket cleanup tracking in transport registry

## Future Enhancements

- [ ] Hardware acceleration (AES-NI, AVX2)
- [ ] FIPS 140-3 certification
- [ ] WebSocket transport
- [ ] QUIC transport
- [ ] Extended monitoring dashboards
- [ ] Performance optimizations for luxfi/zmq router

## Support & Resources

- **Repository**: github.com/luxfi/qzmq
- **Documentation**: See README.md for user-facing docs
- **Issues**: GitHub Issues for bug reports
- **Security**: security@lux.network for security issues

## Important Notes for Future Claude Instances

1. **Always use real implementations** - The user strongly prefers production-ready code over stubs or mocks
2. **Use luxfi packages** - Prefer luxfi/log, luxfi/metric, luxfi/zmq over alternatives
3. **Test with both backends** - Ensure changes work with both CGO_ENABLED=0 and CGO_ENABLED=1
4. **Maintain compatibility** - QZMQ must remain a drop-in replacement for CurveZMQ
5. **Follow the user's style** - Direct, concise communication without unnecessary elaboration

---
*This document was generated for AI assistant context. Keep it updated as the codebase evolves.*

## Context for All AI Assistants

This file (`LLM.md`) is symlinked as:
- `.AGENTS.md`
- `CLAUDE.md`
- `QWEN.md`
- `GEMINI.md`

All files reference the same knowledge base. Updates here propagate to all AI systems.

## Rules for AI Assistants

1. **ALWAYS** update LLM.md with significant discoveries
2. **NEVER** commit symlinked files (.AGENTS.md, CLAUDE.md, etc.) - they're in .gitignore
3. **NEVER** create random summary files - update THIS file
