#!/usr/bin/env bash

# QZMQ Monitoring Script
# Monitors QZMQ metrics and health

set -euo pipefail

# Configuration
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
ALERT_THRESHOLD_ERRORS="${ALERT_THRESHOLD_ERRORS:-100}"
ALERT_THRESHOLD_LATENCY="${ALERT_THRESHOLD_LATENCY:-1.0}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to query Prometheus
query_prometheus() {
    local query="$1"
    curl -s -G "${PROMETHEUS_URL}/api/v1/query" \
        --data-urlencode "query=${query}" | \
        jq -r '.data.result[0].value[1] // 0'
}

# Function to check metric
check_metric() {
    local metric="$1"
    local threshold="$2"
    local operator="$3"
    local description="$4"
    
    value=$(query_prometheus "$metric")
    
    case "$operator" in
        "gt")
            if (( $(echo "$value > $threshold" | bc -l) )); then
                echo -e "${RED}[ALERT]${NC} $description: $value (threshold: $threshold)"
                return 1
            else
                echo -e "${GREEN}[OK]${NC} $description: $value"
            fi
            ;;
        "lt")
            if (( $(echo "$value < $threshold" | bc -l) )); then
                echo -e "${RED}[ALERT]${NC} $description: $value (threshold: $threshold)"
                return 1
            else
                echo -e "${GREEN}[OK]${NC} $description: $value"
            fi
            ;;
    esac
    return 0
}

# Main monitoring loop
main() {
    echo "=== QZMQ Monitoring Dashboard ==="
    echo "Prometheus: ${PROMETHEUS_URL}"
    echo "Grafana: ${GRAFANA_URL}"
    echo ""
    
    while true; do
        clear
        echo "=== QZMQ Metrics Report - $(date) ==="
        echo ""
        
        # Message metrics
        echo "📨 Message Statistics:"
        messages_sent=$(query_prometheus 'sum(rate(qzmq_messages_total{direction="sent"}[5m]))')
        messages_recv=$(query_prometheus 'sum(rate(qzmq_messages_total{direction="received"}[5m]))')
        bytes_sent=$(query_prometheus 'sum(rate(qzmq_messages_bytes{direction="sent"}[5m]))')
        bytes_recv=$(query_prometheus 'sum(rate(qzmq_messages_bytes{direction="received"}[5m]))')
        
        echo "  Messages/sec - Sent: ${messages_sent}, Received: ${messages_recv}"
        echo "  Bytes/sec - Sent: ${bytes_sent}, Received: ${bytes_recv}"
        echo ""
        
        # Connection metrics
        echo "🔌 Connection Statistics:"
        active_connections=$(query_prometheus 'sum(qzmq_connections_active)')
        failed_connections=$(query_prometheus 'sum(rate(qzmq_connections_total{status="failed"}[5m]))')
        
        echo "  Active connections: ${active_connections}"
        echo "  Failed connections/min: ${failed_connections}"
        echo ""
        
        # Crypto metrics
        echo "🔐 Cryptographic Operations:"
        kem_ops=$(query_prometheus 'sum(rate(qzmq_crypto_kem_operations_total[5m]))')
        sign_ops=$(query_prometheus 'sum(rate(qzmq_crypto_sign_operations_total[5m]))')
        aead_ops=$(query_prometheus 'sum(rate(qzmq_crypto_aead_operations_total[5m]))')
        key_rotations=$(query_prometheus 'sum(rate(qzmq_security_key_rotations_total[5m]))')
        
        echo "  KEM ops/sec: ${kem_ops}"
        echo "  Sign ops/sec: ${sign_ops}"
        echo "  AEAD ops/sec: ${aead_ops}"
        echo "  Key rotations/min: ${key_rotations}"
        echo ""
        
        # Performance metrics
        echo "⚡ Performance Metrics:"
        avg_latency=$(query_prometheus 'histogram_quantile(0.5, sum(rate(qzmq_performance_message_latency_seconds_bucket[5m])) by (le))')
        p99_latency=$(query_prometheus 'histogram_quantile(0.99, sum(rate(qzmq_performance_message_latency_seconds_bucket[5m])) by (le))')
        handshake_time=$(query_prometheus 'histogram_quantile(0.5, sum(rate(qzmq_handshake_duration_seconds_bucket[5m])) by (le))')
        
        echo "  Median message latency: ${avg_latency}s"
        echo "  P99 message latency: ${p99_latency}s"
        echo "  Median handshake time: ${handshake_time}s"
        echo ""
        
        # Error metrics
        echo "❌ Error Statistics:"
        error_rate=$(query_prometheus 'sum(rate(qzmq_errors_total[5m]))')
        
        check_metric "sum(rate(qzmq_errors_total[5m]))" "$ALERT_THRESHOLD_ERRORS" "gt" "Error rate"
        echo ""
        
        # Health checks
        echo "🏥 Health Checks:"
        check_metric "histogram_quantile(0.99, sum(rate(qzmq_performance_message_latency_seconds_bucket[5m])) by (le))" "$ALERT_THRESHOLD_LATENCY" "gt" "P99 Latency"
        check_metric "sum(qzmq_connections_active)" "1" "lt" "Active connections"
        echo ""
        
        # Socket type breakdown
        echo "📊 Socket Type Breakdown:"
        for socket_type in REQ REP DEALER ROUTER PUB SUB PUSH PULL PAIR; do
            count=$(query_prometheus "sum(qzmq_connections_active{socket_type=\"${socket_type}\"})")
            if [ "$count" != "0" ]; then
                echo "  ${socket_type}: ${count} active"
            fi
        done
        echo ""
        
        echo "Press Ctrl+C to exit, refreshing in 10 seconds..."
        sleep 10
    done
}

# Signal handler for clean exit
trap 'echo -e "\n${YELLOW}Monitoring stopped${NC}"; exit 0' INT TERM

# Check dependencies
for cmd in curl jq bc; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: $cmd is required but not installed."
        exit 1
    fi
done

# Run main monitoring loop
main