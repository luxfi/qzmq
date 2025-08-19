package qzmq

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestTransportCreation(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "default options",
			opts:    DefaultOptions(),
			wantErr: false,
		},
		{
			name:    "conservative options",
			opts:    ConservativeOptions(),
			wantErr: false,
		},
		{
			name:    "performance options",
			opts:    PerformanceOptions(),
			wantErr: false,
		},
		{
			name: "custom options",
			opts: Options{
				Backend: BackendGo,
				Suite: Suite{
					KEM:  X25519,
					Sign: Ed25519,
					AEAD: ChaCha20Poly1305,
					Hash: SHA256,
				},
				Mode: ModeHybrid,
			},
			wantErr: false,
		},
		{
			name: "invalid PQ-only with classical KEM",
			opts: Options{
				Backend: BackendGo,
				Suite: Suite{
					KEM:  X25519,
					Sign: MLDSA2,
					AEAD: AES256GCM,
					Hash: SHA256,
				},
				Mode: ModePQOnly,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := New(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if transport != nil {
				defer transport.Close()
			}
		})
	}
}

func TestSocketCreation(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	socketTypes := []SocketType{
		REQ, REP, DEALER, ROUTER, PUB, SUB, XPUB, XSUB, PUSH, PULL, PAIR, STREAM,
	}

	for _, st := range socketTypes {
		t.Run(st.String(), func(t *testing.T) {
			socket, err := transport.NewSocket(st)
			if err != nil {
				t.Errorf("Failed to create socket type %v: %v", st, err)
				return
			}
			defer socket.Close()

			// Verify socket is created
			if socket == nil {
				t.Error("Socket is nil")
			}
		})
	}
}

func TestReqRepPattern(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create REP socket
	repSocket, err := transport.NewSocket(REP)
	if err != nil {
		t.Fatalf("Failed to create REP socket: %v", err)
	}
	defer repSocket.Close()

	// Bind REP socket
	if err := repSocket.Bind("tcp://127.0.0.1:15555"); err != nil {
		t.Fatalf("Failed to bind REP socket: %v", err)
	}

	// Create REQ socket
	reqSocket, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatalf("Failed to create REQ socket: %v", err)
	}
	defer reqSocket.Close()

	// Connect REQ socket
	if err := reqSocket.Connect("tcp://127.0.0.1:15555"); err != nil {
		t.Fatalf("Failed to connect REQ socket: %v", err)
	}

	// Allow connection to establish
	time.Sleep(100 * time.Millisecond)

	// Test message exchange
	testMsg := []byte("Hello QZMQ")
	replyMsg := []byte("Hello Client")

	// Server goroutine
	done := make(chan bool)
	go func() {
		msg, err := repSocket.Recv()
		if err != nil {
			t.Errorf("REP recv error: %v", err)
			done <- false
			return
		}
		if !bytes.Equal(msg, testMsg) {
			t.Errorf("REP received wrong message: got %s, want %s", msg, testMsg)
			done <- false
			return
		}
		if err := repSocket.Send(replyMsg); err != nil {
			t.Errorf("REP send error: %v", err)
			done <- false
			return
		}
		done <- true
	}()

	// Client sends request
	if err := reqSocket.Send(testMsg); err != nil {
		t.Fatalf("REQ send error: %v", err)
	}

	// Client receives reply
	reply, err := reqSocket.Recv()
	if err != nil {
		t.Fatalf("REQ recv error: %v", err)
	}

	if !bytes.Equal(reply, replyMsg) {
		t.Errorf("REQ received wrong reply: got %s, want %s", reply, replyMsg)
	}

	// Wait for server to complete
	success := <-done
	if !success {
		t.Fatal("Server goroutine failed")
	}
}

func TestPubSubPattern(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create PUB socket
	pubSocket, err := transport.NewSocket(PUB)
	if err != nil {
		t.Fatalf("Failed to create PUB socket: %v", err)
	}
	defer pubSocket.Close()

	// Bind PUB socket
	if err := pubSocket.Bind("tcp://127.0.0.1:15556"); err != nil {
		t.Fatalf("Failed to bind PUB socket: %v", err)
	}

	// Create SUB socket
	subSocket, err := transport.NewSocket(SUB)
	if err != nil {
		t.Fatalf("Failed to create SUB socket: %v", err)
	}
	defer subSocket.Close()

	// Subscribe to all messages
	if err := subSocket.Subscribe(""); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Connect SUB socket
	if err := subSocket.Connect("tcp://127.0.0.1:15556"); err != nil {
		t.Fatalf("Failed to connect SUB socket: %v", err)
	}

	// Allow subscription to propagate
	time.Sleep(100 * time.Millisecond)

	// Publish messages
	messages := [][]byte{
		[]byte("Message 1"),
		[]byte("Message 2"),
		[]byte("Message 3"),
	}

	for _, msg := range messages {
		if err := pubSocket.Send(msg); err != nil {
			t.Fatalf("PUB send error: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay between messages
	}

	// Receive messages
	for i, expected := range messages {
		msg, err := subSocket.Recv()
		if err != nil {
			t.Fatalf("SUB recv error on message %d: %v", i, err)
		}
		if !bytes.Equal(msg, expected) {
			t.Errorf("SUB received wrong message %d: got %s, want %s", i, msg, expected)
		}
	}
}

func TestMultipartMessages(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create DEALER socket
	dealer, err := transport.NewSocket(DEALER)
	if err != nil {
		t.Fatalf("Failed to create DEALER socket: %v", err)
	}
	defer dealer.Close()

	// Create ROUTER socket
	router, err := transport.NewSocket(ROUTER)
	if err != nil {
		t.Fatalf("Failed to create ROUTER socket: %v", err)
	}
	defer router.Close()

	// Bind ROUTER
	if err := router.Bind("tcp://127.0.0.1:15557"); err != nil {
		t.Fatalf("Failed to bind ROUTER: %v", err)
	}

	// Connect DEALER
	if err := dealer.Connect("tcp://127.0.0.1:15557"); err != nil {
		t.Fatalf("Failed to connect DEALER: %v", err)
	}

	// Allow connection to establish
	time.Sleep(100 * time.Millisecond)

	// Send multipart message
	parts := [][]byte{
		[]byte("Part1"),
		[]byte("Part2"),
		[]byte("Part3"),
	}

	done := make(chan bool)
	go func() {
		recvParts, err := router.RecvMultipart()
		if err != nil {
			t.Errorf("ROUTER recv multipart error: %v", err)
			done <- false
			return
		}
		// Router adds identity frame, so we expect one more part
		if len(recvParts) != len(parts)+1 {
			t.Errorf("Wrong number of parts: got %d, want %d", len(recvParts), len(parts)+1)
			done <- false
			return
		}
		done <- true
	}()

	if err := dealer.SendMultipart(parts); err != nil {
		t.Fatalf("DEALER send multipart error: %v", err)
	}

	success := <-done
	if !success {
		t.Fatal("Router receive failed")
	}
}

func TestTransportStats(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Get initial stats
	stats := transport.Stats()
	if stats == nil {
		t.Fatal("Stats is nil")
	}

	// Create sockets and exchange messages
	rep, _ := transport.NewSocket(REP)
	rep.Bind("tcp://127.0.0.1:15558")
	defer rep.Close()

	req, _ := transport.NewSocket(REQ)
	req.Connect("tcp://127.0.0.1:15558")
	defer req.Close()

	time.Sleep(100 * time.Millisecond)

	// Exchange messages
	go func() {
		msg, _ := rep.Recv()
		rep.Send(msg)
	}()

	req.Send([]byte("test"))
	req.Recv()

	// Get updated stats
	newStats := transport.Stats()

	// Some stats should have increased
	if newStats.MessagesEncrypted == stats.MessagesEncrypted {
		t.Log("Warning: MessagesEncrypted did not increase")
	}
}

func TestConcurrentSockets(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	numSockets := 10
	var wg sync.WaitGroup
	wg.Add(numSockets)

	for i := 0; i < numSockets; i++ {
		go func(id int) {
			defer wg.Done()

			socket, err := transport.NewSocket(REQ)
			if err != nil {
				t.Errorf("Failed to create socket %d: %v", id, err)
				return
			}
			defer socket.Close()

			// Verify socket works
			metrics := socket.GetMetrics()
			if metrics == nil {
				t.Errorf("Socket %d metrics is nil", id)
			}
		}(i)
	}

	wg.Wait()
}

func TestSocketOptions(t *testing.T) {
	transport, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	socket, err := transport.NewSocket(REQ)
	if err != nil {
		t.Fatalf("Failed to create socket: %v", err)
	}
	defer socket.Close()

	// Test setting options
	if err := socket.SetOption("sndhwm", 5000); err != nil {
		t.Errorf("Failed to set sndhwm: %v", err)
	}

	// Test getting options
	val, err := socket.GetOption("qzmq.encrypted")
	if err != nil {
		t.Errorf("Failed to get qzmq.encrypted: %v", err)
	}

	if encrypted, ok := val.(bool); ok {
		t.Logf("Socket encrypted: %v", encrypted)
	}

	// Get suite info
	suite, err := socket.GetOption("qzmq.suite")
	if err != nil {
		t.Errorf("Failed to get qzmq.suite: %v", err)
	}
	if suite == nil {
		t.Error("Suite is nil")
	}
}

// String methods for testing
func (st SocketType) String() string {
	switch st {
	case REQ:
		return "REQ"
	case REP:
		return "REP"
	case DEALER:
		return "DEALER"
	case ROUTER:
		return "ROUTER"
	case PUB:
		return "PUB"
	case SUB:
		return "SUB"
	case XPUB:
		return "XPUB"
	case XSUB:
		return "XSUB"
	case PUSH:
		return "PUSH"
	case PULL:
		return "PULL"
	case PAIR:
		return "PAIR"
	case STREAM:
		return "STREAM"
	default:
		return "UNKNOWN"
	}
}
