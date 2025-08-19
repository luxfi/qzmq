#!/usr/bin/env bash

# QZMQ Deployment Script
# Deploys QZMQ with proper configuration

set -euo pipefail

# Configuration
DEPLOY_ENV="${1:-development}"
CONFIG_DIR="${CONFIG_DIR:-/etc/qzmq}"
LOG_DIR="${LOG_DIR:-/var/log/qzmq}"
METRICS_PORT="${METRICS_PORT:-9090}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed"
        exit 1
    fi
    
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    required_version="1.21"
    if [ "$(printf '%s\n' "$required_version" "$go_version" | sort -V | head -n1)" != "$required_version" ]; then
        print_error "Go version $go_version is too old. Required: $required_version+"
        exit 1
    fi
    print_success "Go version $go_version OK"
    
    # Check for required tools
    for tool in git make curl; do
        if ! command -v $tool &> /dev/null; then
            print_error "$tool is not installed"
            exit 1
        fi
    done
    print_success "All required tools installed"
}

# Function to build QZMQ
build_qzmq() {
    print_info "Building QZMQ..."
    
    # Build with appropriate tags
    case "$DEPLOY_ENV" in
        production)
            print_info "Building production version with CGO enabled..."
            CGO_ENABLED=1 go build -tags "czmq" -ldflags="-s -w" -o bin/qzmq ./cmd/qzmq
            ;;
        staging)
            print_info "Building staging version..."
            go build -o bin/qzmq ./cmd/qzmq
            ;;
        development)
            print_info "Building development version with debug symbols..."
            go build -gcflags="all=-N -l" -o bin/qzmq ./cmd/qzmq
            ;;
        *)
            print_error "Unknown environment: $DEPLOY_ENV"
            exit 1
            ;;
    esac
    
    print_success "Build complete"
}

# Function to generate configuration
generate_config() {
    print_info "Generating configuration for $DEPLOY_ENV..."
    
    mkdir -p "$CONFIG_DIR"
    
    cat > "$CONFIG_DIR/qzmq.yaml" <<EOF
# QZMQ Configuration
environment: $DEPLOY_ENV

# Security settings
security:
  mode: $([ "$DEPLOY_ENV" = "production" ] && echo "pq_only" || echo "hybrid")
  suite:
    kem: "x25519+mlkem768"
    sign: "mldsa87"
    aead: "aes256gcm"
    hash: "sha256"
  key_rotation:
    max_messages: 4294967296
    max_bytes: 1125899906842624
    max_age_seconds: 600

# Network settings
network:
  listen_address: "0.0.0.0"
  port: 5555
  max_connections: 10000
  heartbeat_interval: 30

# Performance settings
performance:
  worker_threads: $(nproc)
  io_threads: 4
  max_sockets: 65536
  send_buffer_size: 262144
  recv_buffer_size: 262144

# Logging
logging:
  level: $([ "$DEPLOY_ENV" = "production" ] && echo "info" || echo "debug")
  output: "$LOG_DIR/qzmq.log"
  max_size: 100  # MB
  max_backups: 10
  max_age: 30    # days

# Metrics
metrics:
  enabled: true
  port: $METRICS_PORT
  path: "/metrics"
  
# High availability
ha:
  enabled: $([ "$DEPLOY_ENV" = "production" ] && echo "true" || echo "false")
  cluster_size: 3
  consensus_timeout: 5000
EOF
    
    print_success "Configuration generated at $CONFIG_DIR/qzmq.yaml"
}

# Function to setup systemd service
setup_systemd() {
    print_info "Setting up systemd service..."
    
    if [ "$EUID" -ne 0 ]; then
        print_warning "Not running as root, skipping systemd setup"
        return
    fi
    
    cat > /etc/systemd/system/qzmq.service <<EOF
[Unit]
Description=QZMQ Post-Quantum Transport
After=network.target
Documentation=https://github.com/luxfi/qzmq

[Service]
Type=simple
User=qzmq
Group=qzmq
ExecStart=/usr/local/bin/qzmq --config $CONFIG_DIR/qzmq.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=qzmq

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$LOG_DIR

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
    
    # Create user if not exists
    if ! id -u qzmq &>/dev/null; then
        useradd -r -s /bin/false qzmq
    fi
    
    # Create directories
    mkdir -p "$LOG_DIR"
    chown qzmq:qzmq "$LOG_DIR"
    
    # Install binary
    cp bin/qzmq /usr/local/bin/
    chmod +x /usr/local/bin/qzmq
    
    # Reload systemd
    systemctl daemon-reload
    
    print_success "Systemd service configured"
}

# Function to setup monitoring
setup_monitoring() {
    print_info "Setting up monitoring..."
    
    # Install Prometheus configuration
    if [ -d "/etc/prometheus" ]; then
        cat > /etc/prometheus/qzmq.yml <<EOF
- job_name: 'qzmq'
  static_configs:
    - targets: ['localhost:$METRICS_PORT']
      labels:
        environment: '$DEPLOY_ENV'
EOF
        print_success "Prometheus configuration added"
    else
        print_warning "Prometheus not found, skipping monitoring setup"
    fi
    
    # Create Grafana dashboard
    if [ -d "/var/lib/grafana/dashboards" ]; then
        cp scripts/grafana-dashboard.json /var/lib/grafana/dashboards/qzmq.json
        print_success "Grafana dashboard installed"
    fi
}

# Function to run tests
run_tests() {
    print_info "Running tests..."
    
    # Run unit tests
    go test -v -race ./...
    if [ $? -ne 0 ]; then
        print_error "Tests failed"
        exit 1
    fi
    
    # Run benchmarks
    go test -bench=. -benchmem ./...
    
    print_success "All tests passed"
}

# Function to validate deployment
validate_deployment() {
    print_info "Validating deployment..."
    
    # Check if service is running (if systemd was setup)
    if [ "$EUID" -eq 0 ] && systemctl is-active --quiet qzmq; then
        print_success "QZMQ service is running"
    fi
    
    # Check metrics endpoint
    if curl -s "http://localhost:$METRICS_PORT/metrics" | grep -q "qzmq_"; then
        print_success "Metrics endpoint is accessible"
    else
        print_warning "Metrics endpoint not accessible"
    fi
    
    # Check configuration
    if [ -f "$CONFIG_DIR/qzmq.yaml" ]; then
        print_success "Configuration file exists"
    else
        print_error "Configuration file missing"
    fi
}

# Main deployment flow
main() {
    echo -e "${BLUE}=== QZMQ Deployment Script ===${NC}"
    echo "Environment: $DEPLOY_ENV"
    echo ""
    
    check_prerequisites
    
    if [ "$2" != "--skip-tests" ]; then
        run_tests
    fi
    
    build_qzmq
    generate_config
    
    if [ "$EUID" -eq 0 ]; then
        setup_systemd
        setup_monitoring
    else
        print_warning "Running as non-root, skipping system integration"
    fi
    
    validate_deployment
    
    echo ""
    print_success "Deployment complete!"
    echo ""
    echo "Next steps:"
    if [ "$EUID" -eq 0 ]; then
        echo "  1. Start the service: systemctl start qzmq"
        echo "  2. Enable auto-start: systemctl enable qzmq"
        echo "  3. Check logs: journalctl -u qzmq -f"
    else
        echo "  1. Run manually: ./bin/qzmq --config $CONFIG_DIR/qzmq.yaml"
        echo "  2. Check logs: tail -f $LOG_DIR/qzmq.log"
    fi
    echo "  3. Monitor metrics: http://localhost:$METRICS_PORT/metrics"
    echo "  4. Run monitoring: ./scripts/monitor.sh"
}

# Parse arguments
if [ $# -eq 0 ]; then
    print_info "Usage: $0 <environment> [--skip-tests]"
    print_info "Environments: development, staging, production"
    exit 1
fi

# Run main deployment
main "$@"