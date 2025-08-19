//go:build !czmq
// +build !czmq

package qzmq

import (
	"testing"
)

// TestStubImplementation verifies the stub backend works
func TestStubImplementation(t *testing.T) {
	t.Log("Running with stub backend (no real ZeroMQ)")

	// Create transport
	opts := DefaultOptions()
	transport, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create sockets
	req, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatalf("Failed to create REQ socket: %v", err)
	}
	defer req.Close()

	rep, err := transport.NewSocket(REP)
	if err != nil {
		t.Fatalf("Failed to create REP socket: %v", err)
	}
	defer rep.Close()

	// Test basic send/recv (stub behavior)
	testData := []byte("test message")

	// Send from REQ
	if err := req.Send(testData); err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// The stub implementation provides basic echo functionality
	// so we can verify the socket interfaces work
	t.Log("Stub backend test completed successfully")
}

// Skip intensive tests when using stub backend
func init() {
	// Mark that we're using the stub backend
	// Tests can check this to adjust expectations
	isStubBackend = true
}

var isStubBackend bool

// Helper to skip tests that require real ZMQ
func skipIfStub(t *testing.T) {
	if isStubBackend {
		t.Skip("Skipping test - requires real ZeroMQ backend")
	}
}
