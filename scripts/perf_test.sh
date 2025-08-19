#!/usr/bin/env bash

# QZMQ Performance Testing Script
# Tests throughput, latency, and scalability

set -euo pipefail

# Configuration
TEST_DURATION="${TEST_DURATION:-30}"
MESSAGE_SIZE="${MESSAGE_SIZE:-1024}"
NUM_CLIENTS="${NUM_CLIENTS:-10}"
TARGET_THROUGHPUT="${TARGET_THROUGHPUT:-10000}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Results storage
RESULTS_DIR="perf_results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

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

# Function to run throughput test
test_throughput() {
    print_info "Testing throughput with $1 pattern..."
    local pattern=$1
    local output_file="$RESULTS_DIR/throughput_${pattern}.txt"
    
    case "$pattern" in
        "req-rep")
            go run cmd/perf/throughput.go \
                --pattern req-rep \
                --duration "$TEST_DURATION" \
                --message-size "$MESSAGE_SIZE" \
                --clients "$NUM_CLIENTS" \
                > "$output_file" 2>&1
            ;;
        "pub-sub")
            go run cmd/perf/throughput.go \
                --pattern pub-sub \
                --duration "$TEST_DURATION" \
                --message-size "$MESSAGE_SIZE" \
                --subscribers "$NUM_CLIENTS" \
                > "$output_file" 2>&1
            ;;
        "push-pull")
            go run cmd/perf/throughput.go \
                --pattern push-pull \
                --duration "$TEST_DURATION" \
                --message-size "$MESSAGE_SIZE" \
                --workers "$NUM_CLIENTS" \
                > "$output_file" 2>&1
            ;;
        "dealer-router")
            go run cmd/perf/throughput.go \
                --pattern dealer-router \
                --duration "$TEST_DURATION" \
                --message-size "$MESSAGE_SIZE" \
                --dealers "$NUM_CLIENTS" \
                > "$output_file" 2>&1
            ;;
    esac
    
    # Extract results
    local throughput=$(grep "Throughput:" "$output_file" | awk '{print $2}')
    local avg_latency=$(grep "Avg Latency:" "$output_file" | awk '{print $3}')
    local p99_latency=$(grep "P99 Latency:" "$output_file" | awk '{print $3}')
    
    echo "  Throughput: ${throughput} msg/s"
    echo "  Avg Latency: ${avg_latency} ms"
    echo "  P99 Latency: ${p99_latency} ms"
    
    # Check if meets target
    if [ "${throughput%.*}" -ge "$TARGET_THROUGHPUT" ]; then
        print_success "Meets target throughput (${TARGET_THROUGHPUT} msg/s)"
    else
        print_warning "Below target throughput (${TARGET_THROUGHPUT} msg/s)"
    fi
}

# Function to run latency test
test_latency() {
    print_info "Testing latency distribution..."
    local output_file="$RESULTS_DIR/latency_distribution.txt"
    
    go run cmd/perf/latency.go \
        --samples 10000 \
        --message-size "$MESSAGE_SIZE" \
        --pattern req-rep \
        > "$output_file" 2>&1
    
    # Generate latency histogram
    cat "$output_file" | grep "^[0-9]" | \
        awk '{print int($1/100)*100}' | \
        sort -n | uniq -c | \
        awk '{printf "%5d-%5d ms: %s\n", $2, $2+99, $1}' \
        > "$RESULTS_DIR/latency_histogram.txt"
    
    print_success "Latency distribution saved to $RESULTS_DIR/latency_histogram.txt"
}

# Function to run scalability test
test_scalability() {
    print_info "Testing scalability..."
    local output_file="$RESULTS_DIR/scalability.txt"
    
    echo "Clients,Throughput,AvgLatency,P99Latency" > "$output_file"
    
    for clients in 1 5 10 25 50 100 200; do
        print_info "Testing with $clients clients..."
        
        result=$(go run cmd/perf/throughput.go \
            --pattern req-rep \
            --duration 10 \
            --message-size "$MESSAGE_SIZE" \
            --clients "$clients" 2>&1 | \
            grep -E "Throughput:|Avg Latency:|P99 Latency:" | \
            awk '{print $NF}' | tr '\n' ',' | sed 's/,$//')
        
        echo "$clients,$result" >> "$output_file"
    done
    
    print_success "Scalability results saved to $output_file"
}

# Function to run memory test
test_memory() {
    print_info "Testing memory usage..."
    local output_file="$RESULTS_DIR/memory_usage.txt"
    
    # Start server with memory profiling
    go run -memprofile="$RESULTS_DIR/mem.prof" cmd/perf/server.go &
    SERVER_PID=$!
    
    sleep 2
    
    # Run load test
    go run cmd/perf/throughput.go \
        --pattern req-rep \
        --duration "$TEST_DURATION" \
        --message-size "$MESSAGE_SIZE" \
        --clients "$NUM_CLIENTS" \
        > /dev/null 2>&1
    
    # Capture memory stats
    go tool pprof -text "$RESULTS_DIR/mem.prof" > "$output_file"
    
    kill $SERVER_PID 2>/dev/null || true
    
    print_success "Memory profile saved to $output_file"
}

# Function to run crypto performance test
test_crypto() {
    print_info "Testing cryptographic performance..."
    local output_file="$RESULTS_DIR/crypto_perf.txt"
    
    echo "=== Crypto Performance Test ===" > "$output_file"
    echo "" >> "$output_file"
    
    # Test different crypto suites
    for suite in "x25519" "mlkem768" "hybrid"; do
        print_info "Testing $suite..."
        echo "Suite: $suite" >> "$output_file"
        
        go test -bench "Benchmark.*$suite" -benchmem -benchtime=10s ./... | \
            grep -E "^Benchmark" >> "$output_file"
        
        echo "" >> "$output_file"
    done
    
    print_success "Crypto performance results saved to $output_file"
}

# Function to run stress test
test_stress() {
    print_info "Running stress test..."
    local output_file="$RESULTS_DIR/stress_test.txt"
    
    # Start monitoring
    ./scripts/monitor.sh > "$RESULTS_DIR/stress_monitor.log" 2>&1 &
    MONITOR_PID=$!
    
    # Run stress test with increasing load
    for load in 100 500 1000 5000 10000; do
        print_info "Stress testing with $load msg/s target..."
        
        go run cmd/perf/stress.go \
            --target-rate "$load" \
            --duration 60 \
            --message-size "$MESSAGE_SIZE" \
            >> "$output_file" 2>&1
        
        sleep 5  # Cool down between tests
    done
    
    kill $MONITOR_PID 2>/dev/null || true
    
    print_success "Stress test results saved to $output_file"
}

# Function to generate report
generate_report() {
    print_info "Generating performance report..."
    
    cat > "$RESULTS_DIR/report.md" <<EOF
# QZMQ Performance Test Report

**Date**: $(date)
**Test Duration**: ${TEST_DURATION}s
**Message Size**: ${MESSAGE_SIZE} bytes
**Number of Clients**: ${NUM_CLIENTS}

## Summary

### Throughput Results

| Pattern | Throughput (msg/s) | Avg Latency (ms) | P99 Latency (ms) |
|---------|-------------------|------------------|------------------|
EOF
    
    for pattern in req-rep pub-sub push-pull dealer-router; do
        if [ -f "$RESULTS_DIR/throughput_${pattern}.txt" ]; then
            throughput=$(grep "Throughput:" "$RESULTS_DIR/throughput_${pattern}.txt" | awk '{print $2}')
            avg_latency=$(grep "Avg Latency:" "$RESULTS_DIR/throughput_${pattern}.txt" | awk '{print $3}')
            p99_latency=$(grep "P99 Latency:" "$RESULTS_DIR/throughput_${pattern}.txt" | awk '{print $3}')
            echo "| $pattern | $throughput | $avg_latency | $p99_latency |" >> "$RESULTS_DIR/report.md"
        fi
    done
    
    cat >> "$RESULTS_DIR/report.md" <<EOF

### Scalability

See \`scalability.txt\` for detailed results.

### Memory Usage

See \`memory_usage.txt\` for memory profile.

### Cryptographic Performance

See \`crypto_perf.txt\` for detailed benchmarks.

## Recommendations

Based on the test results:

1. **Throughput**: $([ "${throughput%.*}" -ge "$TARGET_THROUGHPUT" ] && echo "✅ Meets target" || echo "⚠️ Below target")
2. **Latency**: Check P99 values for your SLA requirements
3. **Scalability**: Review scalability curve for linear scaling
4. **Memory**: Monitor for memory leaks under sustained load

## Test Files

- Raw throughput data: \`throughput_*.txt\`
- Latency distribution: \`latency_distribution.txt\`
- Scalability data: \`scalability.txt\`
- Memory profile: \`memory_usage.txt\`
- Crypto benchmarks: \`crypto_perf.txt\`
- Stress test results: \`stress_test.txt\`
EOF
    
    print_success "Report generated at $RESULTS_DIR/report.md"
}

# Function to run all tests
run_all_tests() {
    echo -e "${BLUE}=== QZMQ Performance Test Suite ===${NC}"
    echo "Configuration:"
    echo "  Test Duration: ${TEST_DURATION}s"
    echo "  Message Size: ${MESSAGE_SIZE} bytes"
    echo "  Number of Clients: ${NUM_CLIENTS}"
    echo "  Target Throughput: ${TARGET_THROUGHPUT} msg/s"
    echo "  Results Directory: ${RESULTS_DIR}"
    echo ""
    
    # Run throughput tests for different patterns
    for pattern in req-rep pub-sub push-pull dealer-router; do
        test_throughput "$pattern"
        echo ""
    done
    
    # Run other tests
    test_latency
    test_scalability
    test_memory
    test_crypto
    
    if [ "${RUN_STRESS:-false}" = "true" ]; then
        test_stress
    fi
    
    # Generate report
    generate_report
    
    echo ""
    print_success "All tests complete! Results saved to ${RESULTS_DIR}/"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --duration)
            TEST_DURATION="$2"
            shift 2
            ;;
        --message-size)
            MESSAGE_SIZE="$2"
            shift 2
            ;;
        --clients)
            NUM_CLIENTS="$2"
            shift 2
            ;;
        --target)
            TARGET_THROUGHPUT="$2"
            shift 2
            ;;
        --stress)
            RUN_STRESS="true"
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --duration SECONDS     Test duration (default: 30)"
            echo "  --message-size BYTES   Message size (default: 1024)"
            echo "  --clients NUMBER       Number of clients (default: 10)"
            echo "  --target THROUGHPUT    Target throughput (default: 10000)"
            echo "  --stress              Run stress tests (default: false)"
            echo "  --help                Show this help message"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if performance test programs exist
if [ ! -d "cmd/perf" ]; then
    print_warning "Performance test programs not found. Creating stubs..."
    mkdir -p cmd/perf
    # Would create stub programs here in a real deployment
fi

# Run tests
run_all_tests