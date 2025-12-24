#!/usr/bin/env bash

# Test Coverage Report for QZMQ

echo "=== QZMQ Test Coverage Report ==="
echo ""

# Core functionality tests
echo "🧪 Core Functionality Tests:"
go test -v -timeout 5s -run "TestTransportOptions|TestSocketMetrics|TestSocketTypes|TestCryptoSuites|TestOptionsPresets|TestErrorCases" 2>&1 | grep -E "^(---|PASS|FAIL)" | sed 's/^/  /'

echo ""
echo "🔐 Cryptography Tests:"
go test -v -timeout 5s -run "TestX25519KEM|TestMLKEM768|TestHybridKEM|TestAEADCreation|TestKeyDerivation|TestCookieGeneration|TestFinishedMAC" 2>&1 | grep -E "^(---|PASS|FAIL)" | sed 's/^/  /'

echo ""
echo "📡 Socket Pattern Tests:"
go test -v -timeout 5s -run "TestReqRepPattern|TestPubSubPattern|TestPairPattern|TestXPubXSub" 2>&1 | grep -E "^(---|PASS|FAIL)" | sed 's/^/  /'

echo ""
echo " Advanced Tests:"
go test -v -timeout 5s -run "TestTransportStats|TestConcurrentSockets|TestSocketOptions|TestSecurityModes" 2>&1 | grep -E "^(---|PASS|FAIL)" | sed 's/^/  /'

echo ""
echo "📊 Coverage Summary:"
go test -cover -timeout 10s -run "Test[^M]" 2>&1 | grep "coverage:" | tail -1

echo ""
echo " Test Summary:"
PASS_COUNT=$(go test -v -timeout 10s -run "Test[^M]" 2>&1 | grep -c "^--- PASS:" || true)
FAIL_COUNT=$(go test -v -timeout 10s -run "Test[^M]" 2>&1 | grep -c "^--- FAIL:" || true)
SKIP_COUNT=$(go test -v -timeout 10s -run "Test[^M]" 2>&1 | grep -c "^--- SKIP:" || true)

echo "  Passed: $PASS_COUNT"
echo "  Failed: $FAIL_COUNT"
echo "  Skipped: $SKIP_COUNT"

echo ""
echo "📈 Detailed Coverage by Package:"
go test -cover ./... -timeout 10s 2>&1 | grep -E "coverage:|ok|FAIL" | head -10

echo ""
echo " Status: $([ $FAIL_COUNT -eq 0 ] && echo "READY FOR DEPLOYMENT" || echo "NEEDS FIXES")"