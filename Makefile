.PHONY: all build test bench clean fmt lint install examples docs

# Variables
GOMOD = GO111MODULE=on
GO = $(GOMOD) go
GOTEST = $(GO) test
GOBUILD = $(GO) build
GOFMT = gofmt
GOLINT = golangci-lint
GOVET = $(GO) vet

# Build flags
LDFLAGS = -ldflags="-s -w"
TESTFLAGS = -v -race -cover
BENCHFLAGS = -bench=. -benchmem

all: fmt lint test build

build:
	@echo "Building QZMQ library..."
	$(GOBUILD) ./...

test:
	@echo "Running tests..."
	$(GOTEST) $(TESTFLAGS) ./...

test-verbose:
	@echo "Running tests with verbose output..."
	$(GOTEST) $(TESTFLAGS) -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench:
	@echo "Running benchmarks..."
	$(GOTEST) $(BENCHFLAGS) ./...

bench-save:
	@echo "Running benchmarks and saving results..."
	$(GOTEST) $(BENCHFLAGS) ./... | tee benchmark.txt

clean:
	@echo "Cleaning..."
	$(GO) clean
	rm -f coverage.out coverage.html benchmark.txt
	rm -rf bin/ dist/
	find examples -type f -name 'main' -delete

fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	$(GO) fmt ./...

lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		$(GOLINT) run; \
	else \
		echo "golangci-lint not installed, running go vet instead"; \
		$(GOVET) ./...; \
	fi

vet:
	@echo "Running go vet..."
	$(GOVET) ./...

install:
	@echo "Installing QZMQ..."
	$(GO) install ./...

deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

deps-update:
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

examples: build
	@echo "Building examples..."
	$(GOBUILD) $(LDFLAGS) -o examples/basic/basic examples/basic/main.go
	@echo "Examples built successfully"

run-example:
	@echo "Running basic example..."
	cd examples/basic && $(GO) run main.go

docs:
	@echo "Generating documentation..."
	@if command -v godoc > /dev/null; then \
		echo "Starting godoc server on http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not installed, please install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed"

check: fmt lint vet test
	@echo "All checks passed!"

# CI/CD targets
ci-test:
	$(GOTEST) -coverprofile=coverage.out -json ./... > test-results.json

ci-lint:
	$(GOLINT) run --out-format=github-actions

# Release targets
tag:
	@echo "Current tags:"
	@git tag -l
	@echo ""
	@read -p "Enter new version tag (e.g., v0.1.0): " tag; \
	git tag -a $$tag -m "Release $$tag"; \
	echo "Tagged as $$tag. Run 'make push-tag' to push to GitHub"

push-tag:
	git push origin --tags

release: check tag
	@echo "Ready for release. Run 'make push-tag' to publish"

# Help target
help:
	@echo "Available targets:"
	@echo "  make build       - Build the library"
	@echo "  make test        - Run tests"
	@echo "  make bench       - Run benchmarks"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make install     - Install the library"
	@echo "  make examples    - Build examples"
	@echo "  make docs        - Generate documentation"
	@echo "  make check       - Run all checks (fmt, lint, vet, test)"
	@echo "  make release     - Prepare for release"
	@echo "  make help        - Show this help message"