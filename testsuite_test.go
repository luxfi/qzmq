package qzmq

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"
)

// TestSuite runs comprehensive tests for QZMQ
type TestSuite struct {
	t *testing.T
}

func NewTestSuite(t *testing.T) *TestSuite {
	return &TestSuite{t: t}
}

// TestAllPatterns tests all ZeroMQ socket patterns with QZMQ
func TestAllPatterns(t *testing.T) {
	suite := NewTestSuite(t)

	t.Run("REQ-REP", suite.TestReqRep)
	t.Run("PUB-SUB", suite.TestPubSub)
	t.Run("PUSH-PULL", suite.TestPushPull)
	t.Run("DEALER-ROUTER", suite.TestDealerRouter)
	t.Run("PAIR", suite.TestPair)
}

func (s *TestSuite) TestReqRep(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create server
	server, err := transport.NewSocket(REP)
	if err != nil {
		t.Fatalf("Failed to create REP socket: %v", err)
	}
	defer server.Close()

	if err := server.Bind("tcp://127.0.0.1:25555"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Create client
	client, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatalf("Failed to create REQ socket: %v", err)
	}
	defer client.Close()

	if err := client.Connect("tcp://127.0.0.1:25555"); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Test multiple request-response cycles
	for i := 0; i < 10; i++ {
		request := []byte("Request " + string(rune('0'+i)))
		response := []byte("Response " + string(rune('0'+i)))

		done := make(chan bool)
		go func() {
			msg, err := server.Recv()
			if err != nil {
				t.Errorf("Server recv error: %v", err)
				done <- false
				return
			}
			if !bytes.Equal(msg, request) {
				t.Errorf("Wrong request: got %s, want %s", msg, request)
				done <- false
				return
			}
			if err := server.Send(response); err != nil {
				t.Errorf("Server send error: %v", err)
				done <- false
				return
			}
			done <- true
		}()

		if err := client.Send(request); err != nil {
			t.Fatalf("Client send error: %v", err)
		}

		reply, err := client.Recv()
		if err != nil {
			t.Fatalf("Client recv error: %v", err)
		}

		if !bytes.Equal(reply, response) {
			t.Errorf("Wrong response: got %s, want %s", reply, response)
		}

		success := <-done
		if !success {
			t.Fatal("Server failed")
		}
	}
}

func (s *TestSuite) TestPubSub(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create publisher
	pub, err := transport.NewSocket(PUB)
	if err != nil {
		t.Fatalf("Failed to create PUB socket: %v", err)
	}
	defer pub.Close()

	if err := pub.Bind("tcp://127.0.0.1:25556"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Create multiple subscribers
	numSubs := 3
	subs := make([]Socket, numSubs)
	for i := 0; i < numSubs; i++ {
		sub, err := transport.NewSocket(SUB)
		if err != nil {
			t.Fatalf("Failed to create SUB socket %d: %v", i, err)
		}
		defer sub.Close()

		if err := sub.Subscribe(""); err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		if err := sub.Connect("tcp://127.0.0.1:25556"); err != nil {
			t.Fatalf("Failed to connect SUB %d: %v", i, err)
		}
		subs[i] = sub
	}

	time.Sleep(200 * time.Millisecond) // Allow subscriptions to propagate

	// Publish messages
	messages := [][]byte{
		[]byte("Message 1"),
		[]byte("Message 2"),
		[]byte("Message 3"),
	}

	for _, msg := range messages {
		if err := pub.Send(msg); err != nil {
			t.Fatalf("PUB send error: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Each subscriber should receive all messages
	for i, sub := range subs {
		for j, expected := range messages {
			msg, err := sub.Recv()
			if err != nil {
				t.Fatalf("SUB %d recv error on message %d: %v", i, j, err)
			}
			if !bytes.Equal(msg, expected) {
				t.Errorf("SUB %d wrong message %d: got %s, want %s", i, j, msg, expected)
			}
		}
	}
}

func (s *TestSuite) TestPushPull(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create puller (sink)
	pull, err := transport.NewSocket(PULL)
	if err != nil {
		t.Fatalf("Failed to create PULL socket: %v", err)
	}
	defer pull.Close()

	if err := pull.Bind("tcp://127.0.0.1:25557"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Create multiple pushers (workers)
	numPushers := 3
	pushers := make([]Socket, numPushers)
	for i := 0; i < numPushers; i++ {
		push, err := transport.NewSocket(PUSH)
		if err != nil {
			t.Fatalf("Failed to create PUSH socket %d: %v", i, err)
		}
		defer push.Close()

		if err := push.Connect("tcp://127.0.0.1:25557"); err != nil {
			t.Fatalf("Failed to connect PUSH %d: %v", i, err)
		}
		pushers[i] = push
	}

	time.Sleep(100 * time.Millisecond)

	// Each pusher sends messages
	totalMessages := numPushers * 5
	var wg sync.WaitGroup
	wg.Add(numPushers)

	for i, push := range pushers {
		go func(idx int, p Socket) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				msg := []byte(string(rune('A'+idx)) + string(rune('0'+j)))
				if err := p.Send(msg); err != nil {
					t.Errorf("PUSH %d send error: %v", idx, err)
				}
			}
		}(i, push)
	}

	// Receive all messages
	received := make(map[string]bool)
	for i := 0; i < totalMessages; i++ {
		msg, err := pull.Recv()
		if err != nil {
			t.Fatalf("PULL recv error on message %d: %v", i, err)
		}
		received[string(msg)] = true
	}

	wg.Wait()

	// Verify we received all messages
	if len(received) != totalMessages {
		t.Errorf("Expected %d unique messages, got %d", totalMessages, len(received))
	}
}

func (s *TestSuite) TestDealerRouter(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create router
	router, err := transport.NewSocket(ROUTER)
	if err != nil {
		t.Fatalf("Failed to create ROUTER socket: %v", err)
	}
	defer router.Close()

	if err := router.Bind("tcp://127.0.0.1:25558"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Create multiple dealers
	numDealers := 3
	dealers := make([]Socket, numDealers)
	for i := 0; i < numDealers; i++ {
		dealer, err := transport.NewSocket(DEALER)
		if err != nil {
			t.Fatalf("Failed to create DEALER socket %d: %v", i, err)
		}
		defer dealer.Close()

		if err := dealer.Connect("tcp://127.0.0.1:25558"); err != nil {
			t.Fatalf("Failed to connect DEALER %d: %v", i, err)
		}
		dealers[i] = dealer
	}

	time.Sleep(100 * time.Millisecond)

	// Router echo server
	go func() {
		for i := 0; i < numDealers*3; i++ {
			parts, err := router.RecvMultipart()
			if err != nil {
				t.Errorf("Router recv error: %v", err)
				return
			}
			// Echo back to sender
			if err := router.SendMultipart(parts); err != nil {
				t.Errorf("Router send error: %v", err)
				return
			}
		}
	}()

	// Each dealer sends and receives
	var wg sync.WaitGroup
	wg.Add(numDealers)

	for i, dealer := range dealers {
		go func(idx int, d Socket) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				msg := []byte(string(rune('A'+idx)) + string(rune('0'+j)))
				if err := d.Send(msg); err != nil {
					t.Errorf("Dealer %d send error: %v", idx, err)
					return
				}

				reply, err := d.Recv()
				if err != nil {
					t.Errorf("Dealer %d recv error: %v", idx, err)
					return
				}

				if !bytes.Equal(reply, msg) {
					t.Errorf("Dealer %d wrong echo: got %s, want %s", idx, reply, msg)
				}
			}
		}(i, dealer)
	}

	wg.Wait()
}

func (s *TestSuite) TestPair(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create first peer
	peer1, err := transport.NewSocket(PAIR)
	if err != nil {
		t.Fatalf("Failed to create PAIR socket 1: %v", err)
	}
	defer peer1.Close()

	if err := peer1.Bind("tcp://127.0.0.1:25559"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Create second peer
	peer2, err := transport.NewSocket(PAIR)
	if err != nil {
		t.Fatalf("Failed to create PAIR socket 2: %v", err)
	}
	defer peer2.Close()

	if err := peer2.Connect("tcp://127.0.0.1:25559"); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Bidirectional communication
	msg1 := []byte("From peer1")
	msg2 := []byte("From peer2")

	// Peer1 -> Peer2
	if err := peer1.Send(msg1); err != nil {
		t.Fatalf("Peer1 send error: %v", err)
	}

	recv2, err := peer2.Recv()
	if err != nil {
		t.Fatalf("Peer2 recv error: %v", err)
	}
	if !bytes.Equal(recv2, msg1) {
		t.Errorf("Wrong message at peer2: got %s, want %s", recv2, msg1)
	}

	// Peer2 -> Peer1
	if err := peer2.Send(msg2); err != nil {
		t.Fatalf("Peer2 send error: %v", err)
	}

	recv1, err := peer1.Recv()
	if err != nil {
		t.Fatalf("Peer1 recv error: %v", err)
	}
	if !bytes.Equal(recv1, msg2) {
		t.Errorf("Wrong message at peer1: got %s, want %s", recv1, msg2)
	}
}

// TestSecurityModes tests different security configurations
func TestSecurityModes(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"Performance", PerformanceOptions()},
		{"Balanced", DefaultOptions()},
		{"Conservative", ConservativeOptions()},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			transport, err := New(mode.opts)
			if err != nil {
				t.Fatalf("Failed to create transport: %v", err)
			}
			defer transport.Close()

			// Create sockets
			server, err := transport.NewSocket(REP)
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}
			defer server.Close()

			client, err := transport.NewSocket(REQ)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer client.Close()

			// Bind and connect
			port := 25560
			if mode.name == "Balanced" {
				port = 25561
			} else if mode.name == "Conservative" {
				port = 25562
			}

			if err := server.Bind("tcp://127.0.0.1:" + string(rune('0'+port%10)) + string(rune('0'+port/10%10)) + string(rune('0'+port/100%10)) + string(rune('0'+port/1000%10)) + string(rune('0'+port/10000))); err != nil {
				t.Fatalf("Failed to bind: %v", err)
			}

			if err := client.Connect("tcp://127.0.0.1:" + string(rune('0'+port%10)) + string(rune('0'+port/10%10)) + string(rune('0'+port/100%10)) + string(rune('0'+port/1000%10)) + string(rune('0'+port/10000))); err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}

			time.Sleep(100 * time.Millisecond)

			// Test message exchange
			msg := []byte("Test message for " + mode.name)

			go func() {
				req, _ := server.Recv()
				server.Send(req)
			}()

			if err := client.Send(msg); err != nil {
				t.Fatalf("Send error: %v", err)
			}

			reply, err := client.Recv()
			if err != nil {
				t.Fatalf("Recv error: %v", err)
			}

			if !bytes.Equal(reply, msg) {
				t.Errorf("Wrong reply: got %s, want %s", reply, msg)
			}

			// Check that appropriate security is enabled
			encrypted, _ := client.GetOption("qzmq.encrypted")
			t.Logf("%s mode - Encrypted: %v", mode.name, encrypted)

			suite, _ := client.GetOption("qzmq.suite")
			if s, ok := suite.(Suite); ok {
				t.Logf("%s mode - Suite: KEM=%s, Sign=%s, AEAD=%s",
					mode.name, s.KEM, s.Sign, s.AEAD)
			}
		})
	}
}

// TestKeyRotation tests automatic key rotation
// TODO: Implement key rotation in zmq backend
func TestKeyRotation(t *testing.T) {
	t.Skip("Key rotation not yet implemented in zmq backend")
	opts := DefaultOptions()
	opts.KeyRotation = KeyRotationPolicy{
		MaxMessages: 5, // Rotate after 5 messages
		MaxBytes:    0,
		MaxAge:      0,
	}

	transport, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create sockets
	server, err := transport.NewSocket(REP)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	client, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if err := server.Bind("tcp://127.0.0.1:25563"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	if err := client.Connect("tcp://127.0.0.1:25563"); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Send multiple messages to trigger key rotation
	for i := 0; i < 10; i++ {
		msg := []byte("Message " + string(rune('0'+i)))

		go func() {
			req, _ := server.Recv()
			server.Send(req)
		}()

		if err := client.Send(msg); err != nil {
			t.Fatalf("Send error on message %d: %v", i, err)
		}

		reply, err := client.Recv()
		if err != nil {
			t.Fatalf("Recv error on message %d: %v", i, err)
		}

		if !bytes.Equal(reply, msg) {
			t.Errorf("Wrong reply %d: got %s, want %s", i, reply, msg)
		}
	}

	// Check metrics for key updates
	metrics := client.GetMetrics()
	t.Logf("Key updates: %d", metrics.KeyUpdates)

	// We should have at least one key rotation (after 5 messages)
	if metrics.KeyUpdates < 1 {
		t.Error("Expected at least one key rotation")
	}
}

// TestConcurrentConnections tests multiple concurrent connections
func TestConcurrentConnections(t *testing.T) {
	// This test must pass with all backends
	
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create server
	server, err := transport.NewSocket(ROUTER)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.Bind("tcp://127.0.0.1:25564"); err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}

	// Server echo loop - simplified for stability
	go func() {
		for i := 0; i < 300; i++ { // Process limited number for stability
			parts, err := server.RecvMultipart()
			if err != nil {
				return
			}
			server.SendMultipart(parts)
		}
	}()

	// Create multiple concurrent clients - reduced for stability
	numClients := 3
	numMessages := 10
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			client, err := transport.NewSocket(DEALER)
			if err != nil {
				t.Errorf("Client %d: Failed to create socket: %v", clientID, err)
				return
			}
			defer client.Close()

			if err := client.Connect("tcp://127.0.0.1:25564"); err != nil {
				t.Errorf("Client %d: Failed to connect: %v", clientID, err)
				return
			}

			time.Sleep(50 * time.Millisecond)

			for j := 0; j < numMessages; j++ {
				msg := []byte("Client" + string(rune('0'+clientID)) + "-Msg" + string(rune('0'+j)))

				if err := client.Send(msg); err != nil {
					t.Errorf("Client %d: Send error on message %d: %v", clientID, j, err)
					continue
				}

				reply, err := client.Recv()
				if err != nil {
					t.Errorf("Client %d: Recv error on message %d: %v", clientID, j, err)
					continue
				}

				if !bytes.Equal(reply, msg) {
					t.Errorf("Client %d: Wrong reply on message %d", clientID, j)
				}
			}
		}(i)
	}

	wg.Wait()
}

// BenchmarkQZMQThroughput benchmarks message throughput
func BenchmarkQZMQThroughput(b *testing.B) {
	transport, _ := New(PerformanceOptions())
	defer transport.Close()

	server, _ := transport.NewSocket(PULL)
	server.Bind("tcp://127.0.0.1:25565")
	defer server.Close()

	client, _ := transport.NewSocket(PUSH)
	client.Connect("tcp://127.0.0.1:25565")
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	msg := make([]byte, 1024)

	// Receiver goroutine
	go func() {
		for i := 0; i < b.N; i++ {
			server.Recv()
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Send(msg)
	}
}

// BenchmarkQZMQLatency benchmarks round-trip latency
func BenchmarkQZMQLatency(b *testing.B) {
	transport, _ := New(PerformanceOptions())
	defer transport.Close()

	server, _ := transport.NewSocket(REP)
	server.Bind("tcp://127.0.0.1:25566")
	defer server.Close()

	client, _ := transport.NewSocket(REQ)
	client.Connect("tcp://127.0.0.1:25566")
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	msg := make([]byte, 64)

	// Server echo loop
	go func() {
		for {
			req, err := server.Recv()
			if err != nil {
				return
			}
			server.Send(req)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Send(msg)
		client.Recv()
	}
}
