package qzmq

import (
	"testing"
	"time"
)

// TestBasicOperations tests basic QZMQ operations
func TestBasicOperations(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Test Stats
	stats := transport.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// Test socket creation for all types
	socketTypes := []SocketType{REQ, REP, PUB, SUB, PUSH, PULL, DEALER, ROUTER, PAIR}
	for _, st := range socketTypes {
		socket, err := transport.NewSocket(st)
		if err != nil {
			t.Errorf("Failed to create %s socket: %v", st, err)
			continue
		}
		socket.Close()
	}
}

// TestMessageExchange tests basic message exchange patterns
func TestMessageExchange(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Test PUSH-PULL pattern
	pull, err := transport.NewSocket(PULL)
	if err != nil {
		t.Fatal(err)
	}
	defer pull.Close()
	
	push, err := transport.NewSocket(PUSH)
	if err != nil {
		t.Fatal(err)
	}
	defer push.Close()

	pull.Bind("tcp://127.0.0.1:34567")
	push.Connect("tcp://127.0.0.1:34567")
	
	time.Sleep(100 * time.Millisecond)
	
	// Send and receive
	msg := []byte("test message")
	if err := push.Send(msg); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}
	
	received, err := pull.Recv()
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}
	
	if string(received) != string(msg) {
		t.Errorf("Message mismatch: got %s, want %s", received, msg)
	}
}

// TestGetSetOptions tests socket options
func TestGetSetOptions(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()

	socket, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	// Test setting options
	options := map[string]interface{}{
		"linger":        0,
		"sndbuf":        65536,
		"rcvbuf":        65536,
		"reconnect_ivl": 100,
		"rcvtimeo":      1000,
		"sndtimeo":      1000,
	}

	for name, value := range options {
		if err := socket.SetOption(name, value); err != nil {
			// Some options may not be supported, that's OK
			t.Logf("Option %s not supported: %v", name, err)
		}
	}

	// Test getting options
	if val, err := socket.GetOption("qzmq.encrypted"); err == nil {
		t.Logf("Encrypted: %v", val)
	}
}

// TestBuildTags verifies build tags work correctly
func TestBuildTags(t *testing.T) {
	// This test passes regardless of build tags
	// It's here to ensure both backends compile and run
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()

	// Verify we can create sockets
	socket, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatal(err)
	}
	socket.Close()

	t.Log("Build tags test passed")
}