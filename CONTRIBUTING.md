# Contributing to QZMQ

Thank you for your interest in contributing to QZMQ! This document provides guidelines and instructions for contributing to the project.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:
- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive criticism
- Accept feedback gracefully

## How to Contribute

### Reporting Issues

Before creating an issue, please check if it already exists. When creating a new issue, include:
- Clear title and description
- Steps to reproduce (for bugs)
- Expected vs actual behavior
- System information (OS, Go version, ZeroMQ version)
- Relevant code samples or error messages

### Suggesting Features

Feature requests are welcome! Please provide:
- Clear use case description
- Expected behavior
- Alternative solutions considered
- Potential implementation approach

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Follow the coding style** (run `make fmt` and `make lint`)
3. **Add tests** for new functionality
4. **Update documentation** as needed
5. **Ensure all tests pass** (run `make test`)
6. **Write a clear commit message** following conventional commits

#### Commit Message Format

```
type(scope): subject

body (optional)

footer (optional)
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Test additions or corrections
- `chore`: Maintenance tasks

Example:
```
feat(crypto): add ML-KEM-512 support

Implemented ML-KEM-512 for lightweight post-quantum security.
Added tests and benchmarks for the new algorithm.

Closes #123
```

## Development Setup

### Prerequisites

- Go 1.21 or later
- ZeroMQ 4.0 or later (optional, for C backend)
- Git

### Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/qzmq.git
cd qzmq

# Add upstream remote
git remote add upstream https://github.com/luxfi/qzmq.git

# Install dependencies
make deps

# Install development tools
make dev-setup

# Run tests to verify setup
make test
```

### Development Workflow

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Write code
   - Add tests
   - Update documentation

3. **Test your changes**
   ```bash
   make check  # Runs fmt, lint, vet, and test
   ```

4. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: your feature description"
   ```

5. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**
   - Go to GitHub and create a PR from your fork to upstream/main
   - Fill in the PR template
   - Wait for review

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific tests
go test -v ./crypto/...

# Run with race detector
go test -race ./...
```

### Writing Tests

- Place tests in `*_test.go` files
- Use table-driven tests where appropriate
- Test both success and error cases
- Include benchmarks for performance-critical code

Example test:
```go
func TestKEMEncapsulate(t *testing.T) {
    tests := []struct {
        name    string
        kem     KEM
        wantErr bool
    }{
        {"X25519", &X25519KEM{}, false},
        {"ML-KEM-768", &MLKEM768{}, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pk, sk, err := tt.kem.GenerateKeyPair()
            require.NoError(t, err)
            
            ct, ss, err := tt.kem.Encapsulate(pk)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                require.NotEmpty(t, ct)
                require.NotEmpty(t, ss)
            }
        })
    }
}
```

## Documentation

### Code Documentation

- Add godoc comments to all exported types, functions, and packages
- Include examples in documentation where helpful
- Keep comments clear and concise

Example:
```go
// NewTransport creates a new QZMQ transport with the specified options.
// It automatically detects the best available backend (Go or C) unless
// explicitly specified in options.
//
// Example:
//
//	transport, err := qzmq.New(qzmq.DefaultOptions())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer transport.Close()
func New(opts Options) (Transport, error) {
    // ...
}
```

### README Updates

Update the README when:
- Adding new features
- Changing API
- Adding examples
- Updating requirements

## Performance

### Benchmarking

Add benchmarks for performance-critical code:

```go
func BenchmarkKEMEncapsulate(b *testing.B) {
    kem := &MLKEM768{}
    pk, _, _ := kem.GenerateKeyPair()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _, _ = kem.Encapsulate(pk)
    }
}
```

Run benchmarks:
```bash
make bench
```

### Performance Guidelines

- Minimize allocations in hot paths
- Use sync.Pool for frequently allocated objects
- Profile before optimizing
- Document performance characteristics

## Security

### Reporting Security Issues

**DO NOT** create public issues for security vulnerabilities. Instead, email security@lux.network with:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Security Guidelines

- Never log sensitive data (keys, passwords)
- Always use constant-time comparisons for secrets
- Validate all inputs
- Use secure random number generation
- Follow cryptographic best practices

## Release Process

1. **Version Bump**
   - Update version in relevant files
   - Update CHANGELOG.md

2. **Testing**
   - Run full test suite
   - Test on multiple platforms
   - Run benchmarks

3. **Documentation**
   - Update README if needed
   - Check godoc rendering

4. **Tag Release**
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

## Getting Help

- 📧 Email: dev@lux.network
- 💬 Discord: [Lux Network](https://discord.gg/lux)
- 📖 Documentation: [pkg.go.dev/github.com/luxfi/qzmq](https://pkg.go.dev/github.com/luxfi/qzmq)

## Recognition

Contributors will be recognized in:
- CONTRIBUTORS.md file
- Release notes
- Project documentation

Thank you for contributing to QZMQ!