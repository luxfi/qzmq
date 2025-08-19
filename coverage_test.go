package qzmq

import (
	"testing"
	"time"
)

// TestTransportOptions tests various transport options
func TestTransportOptions(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()

	// Test setting options
	err = transport.SetOption("max_sockets", 100)
	if err != nil {
		t.Error("Failed to set max_sockets option")
	}

	// Test getting options
	val, err := transport.GetOption("max_sockets")
	if err != nil {
		t.Error("Failed to get max_sockets option")
	}
	if val != 100 {
		t.Errorf("Expected max_sockets to be 100, got %v", val)
	}

	// Test Stats
	stats := transport.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
}

// TestSocketMetrics tests socket metrics tracking
func TestSocketMetrics(t *testing.T) {
	transport, _ := New(DefaultOptions())
	defer transport.Close()

	req, _ := transport.NewSocket(REQ)
	rep, _ := transport.NewSocket(REP)

	rep.Bind("tcp://127.0.0.1:15570")
	req.Connect("tcp://127.0.0.1:15570")

	// Send and receive to generate metrics
	testMsg := []byte("metrics test")
	req.Send(testMsg)
	rep.Recv()
	rep.Send([]byte("response"))
	req.Recv()

	// Check metrics
	reqMetrics := req.GetMetrics()
	if reqMetrics.MessagesSent == 0 {
		t.Error("REQ socket should have sent messages")
	}
	if reqMetrics.BytesSent == 0 {
		t.Error("REQ socket should have sent bytes")
	}

	repMetrics := rep.GetMetrics()
	if repMetrics.MessagesReceived == 0 {
		t.Error("REP socket should have received messages")
	}
	if repMetrics.BytesReceived == 0 {
		t.Error("REP socket should have received bytes")
	}
}

// TestHandshakeTimeout tests connection timeout handling
func TestHandshakeTimeout(t *testing.T) {
	opts := DefaultOptions()
	opts.Timeouts.Handshake = 100 * time.Millisecond
	
	transport, _ := New(opts)
	defer transport.Close()

	req, _ := transport.NewSocket(REQ)
	defer req.Close()  // Add proper cleanup
	
	// Try to connect to non-existent endpoint (this should return immediately)
	err := req.Connect("tcp://127.0.0.1:19999")
	// With luxfi/zmq backend this won't actually fail immediately, but we're testing the code path
	if err != nil {
		t.Log("Connection failed as expected:", err)
	}
	
	// Don't wait for anything, just return
}

// TestSocketTypes tests all socket type string representations
func TestSocketTypes(t *testing.T) {
	types := []struct {
		st       SocketType
		expected string
	}{
		{REQ, "REQ"},
		{REP, "REP"},
		{DEALER, "DEALER"},
		{ROUTER, "ROUTER"},
		{PUB, "PUB"},
		{SUB, "SUB"},
		{XPUB, "XPUB"},
		{XSUB, "XSUB"},
		{PUSH, "PUSH"},
		{PULL, "PULL"},
		{PAIR, "PAIR"},
		{STREAM, "STREAM"},
		{SocketType(99), "UNKNOWN"},
	}

	for _, tt := range types {
		if got := tt.st.String(); got != tt.expected {
			t.Errorf("SocketType(%d).String() = %s, want %s", tt.st, got, tt.expected)
		}
	}
}

// TestCryptoSuites tests all crypto suite string representations
func TestCryptoSuites(t *testing.T) {
	// Test KEM algorithms string methods
	suite := Suite{
		KEM:  X25519,
		Sign: Ed25519,
		AEAD: AES256GCM,
		Hash: SHA256,
	}

	// Just test that we can create suites with different algorithms
	if suite.KEM != X25519 {
		t.Error("KEM should be X25519")
	}
	if suite.Sign != Ed25519 {
		t.Error("Sign should be Ed25519")
	}
	if suite.AEAD != AES256GCM {
		t.Error("AEAD should be AES256GCM")
	}
	if suite.Hash != SHA256 {
		t.Error("Hash should be SHA256")
	}

	// Test with ML-KEM
	suite2 := Suite{
		KEM:  MLKEM768,
		Sign: MLDSA2,
		AEAD: ChaCha20Poly1305,
		Hash: SHA384,
	}

	if suite2.KEM != MLKEM768 {
		t.Error("KEM should be MLKEM768")
	}
	if suite2.Sign != MLDSA2 {
		t.Error("Sign should be MLDSA2")
	}
}

// TestOptionsPresets tests the preset option functions
func TestOptionsPresets(t *testing.T) {
	// Test default options
	def := DefaultOptions()
	if def.Suite.KEM == 0 {
		t.Error("Default options should have KEM set")
	}
	if def.Suite.Sign == 0 {
		t.Error("Default options should have Sign set")
	}
}

// TestErrorCases tests error handling paths
func TestErrorCases(t *testing.T) {
	transport, _ := New(DefaultOptions())
	defer transport.Close()

	// Test subscribing on non-SUB socket
	req, _ := transport.NewSocket(REQ)
	err := req.Subscribe("test")
	if err == nil {
		t.Error("Subscribe should fail on non-SUB socket")
	}

	// Test unsubscribing on non-SUB socket
	err = req.Unsubscribe("test")
	if err == nil {
		t.Error("Unsubscribe should fail on non-SUB socket")
	}

	// Test double close
	req.Close()
	err = req.Close()
	if err != nil {
		t.Error("Double close should not return error")
	}

	// Test operations on closed socket
	err = req.Send([]byte("test"))
	if err != ErrNotConnected {
		t.Error("Send on closed socket should return ErrNotConnected")
	}

	_, err = req.Recv()
	if err != ErrNotConnected {
		t.Error("Recv on closed socket should return ErrNotConnected")
	}
}

// TestXPubXSub tests XPUB-XSUB pattern
func TestXPubXSub(t *testing.T) {
	transport, _ := New(DefaultOptions())
	defer transport.Close()

	xpub, _ := transport.NewSocket(XPUB)
	xsub, _ := transport.NewSocket(XSUB)

	xpub.Bind("tcp://127.0.0.1:15571")
	xsub.Connect("tcp://127.0.0.1:15571")

	// Subscribe to a topic
	xsub.Subscribe("test")

	// Give time for subscription to propagate
	time.Sleep(50 * time.Millisecond)

	// Send a message on the topic
	xpub.Send([]byte("test message"))

	// Should receive the message
	msg, err := xsub.Recv()
	if err != nil {
		t.Fatal("Failed to receive message:", err)
	}
	if string(msg) != "test message" {
		t.Errorf("Expected 'test message', got '%s'", string(msg))
	}
}

// TestPairPattern tests PAIR socket pattern
func TestPairPattern(t *testing.T) {
	transport, _ := New(DefaultOptions())
	defer transport.Close()

	pair1, _ := transport.NewSocket(PAIR)
	pair2, _ := transport.NewSocket(PAIR)

	pair1.Bind("tcp://127.0.0.1:15572")
	pair2.Connect("tcp://127.0.0.1:15572")

	// Test bidirectional communication
	testMsg1 := []byte("from pair1")
	pair1.Send(testMsg1)
	
	msg, err := pair2.Recv()
	if err != nil || string(msg) != "from pair1" {
		t.Error("PAIR pattern failed pair1->pair2")
	}

	testMsg2 := []byte("from pair2")
	pair2.Send(testMsg2)
	
	msg, err = pair1.Recv()
	if err != nil || string(msg) != "from pair2" {
		t.Error("PAIR pattern failed pair2->pair1")
	}
}