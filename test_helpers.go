package qzmq

import (
	"fmt"
	"testing"
	"time"
)

// TestContext provides common test utilities
type TestContext struct {
	t         *testing.T
	transport Transport
	sockets   []Socket
}

// NewTestContext creates a new test context
func NewTestContext(t *testing.T, opts Options) *TestContext {
	transport, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	return &TestContext{
		t:         t,
		transport: transport,
		sockets:   make([]Socket, 0),
	}
}

// CreateSocket creates a socket and tracks it for cleanup
func (tc *TestContext) CreateSocket(socketType SocketType) Socket {
	socket, err := tc.transport.NewSocket(socketType)
	if err != nil {
		tc.t.Fatalf("Failed to create %s socket: %v", socketType, err)
	}
	tc.sockets = append(tc.sockets, socket)
	return socket
}

// BindSocket creates, binds, and tracks a socket
func (tc *TestContext) BindSocket(socketType SocketType, port int) Socket {
	socket := tc.CreateSocket(socketType)
	endpoint := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	if err := socket.Bind(endpoint); err != nil {
		tc.t.Fatalf("Failed to bind %s socket to %s: %v", socketType, endpoint, err)
	}
	return socket
}

// ConnectSocket creates, connects, and tracks a socket
func (tc *TestContext) ConnectSocket(socketType SocketType, port int) Socket {
	socket := tc.CreateSocket(socketType)
	endpoint := fmt.Sprintf("tcp://127.0.0.1:%d", port)
	if err := socket.Connect(endpoint); err != nil {
		tc.t.Fatalf("Failed to connect %s socket to %s: %v", socketType, endpoint, err)
	}
	return socket
}

// CreatePair creates a connected socket pair
func (tc *TestContext) CreatePair(serverType, clientType SocketType, port int) (Socket, Socket) {
	server := tc.BindSocket(serverType, port)
	client := tc.ConnectSocket(clientType, port)

	// Special handling for SUB sockets
	if clientType == SUB {
		client.Subscribe("")
	}

	time.Sleep(100 * time.Millisecond) // Allow connection
	return server, client
}

// Cleanup closes all tracked sockets and the transport
func (tc *TestContext) Cleanup() {
	for _, socket := range tc.sockets {
		socket.Close()
	}
	tc.transport.Close()
}

// SendRecv performs a send-receive cycle
func (tc *TestContext) SendRecv(sender, receiver Socket, msg []byte) {
	if err := sender.Send(msg); err != nil {
		tc.t.Fatalf("Send failed: %v", err)
	}

	received, err := receiver.Recv()
	if err != nil {
		tc.t.Fatalf("Recv failed: %v", err)
	}

	if string(received) != string(msg) {
		tc.t.Errorf("Message mismatch: got %s, want %s", received, msg)
	}
}

// EchoServer runs a simple echo server in the background
func (tc *TestContext) EchoServer(socket Socket, done <-chan struct{}) {
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				msg, err := socket.Recv()
				if err != nil {
					return
				}
				socket.Send(msg)
			}
		}
	}()
}

// RouterEchoServer runs a router echo server
func (tc *TestContext) RouterEchoServer(router Socket, done <-chan struct{}) {
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				parts, err := router.RecvMultipart()
				if err != nil {
					return
				}
				router.SendMultipart(parts)
			}
		}
	}()
}

// AssertMetrics checks socket metrics
func (tc *TestContext) AssertMetrics(socket Socket, minMessages, minBytes int) {
	metrics := socket.GetMetrics()
	if metrics.MessagesReceived < uint64(minMessages) {
		tc.t.Errorf("Expected at least %d messages, got %d", minMessages, metrics.MessagesReceived)
	}
	if metrics.BytesReceived < uint64(minBytes) {
		tc.t.Errorf("Expected at least %d bytes, got %d", minBytes, metrics.BytesReceived)
	}
}

// TestPattern runs a standard test for a socket pattern
func TestPattern(t *testing.T, serverType, clientType SocketType, port int) {
	tc := NewTestContext(t, DefaultOptions())
	defer tc.Cleanup()

	server, client := tc.CreatePair(serverType, clientType, port)

	// Test message exchange based on pattern
	switch serverType {
	case REP:
		done := make(chan struct{})
		tc.EchoServer(server, done)
		tc.SendRecv(client, client, []byte("test"))
		close(done)

	case PUB:
		time.Sleep(100 * time.Millisecond) // Allow subscription
		if err := server.Send([]byte("broadcast")); err != nil {
			t.Fatalf("PUB send failed: %v", err)
		}
		msg, err := client.Recv()
		if err != nil {
			t.Fatalf("SUB recv failed: %v", err)
		}
		if string(msg) != "broadcast" {
			t.Errorf("Wrong message: got %s, want broadcast", msg)
		}

	case PULL:
		if err := client.Send([]byte("work")); err != nil {
			t.Fatalf("PUSH send failed: %v", err)
		}
		msg, err := server.Recv()
		if err != nil {
			t.Fatalf("PULL recv failed: %v", err)
		}
		if string(msg) != "work" {
			t.Errorf("Wrong message: got %s, want work", msg)
		}

	case ROUTER:
		done := make(chan struct{})
		tc.RouterEchoServer(server, done)
		tc.SendRecv(client, client, []byte("async"))
		close(done)

	case PAIR:
		tc.SendRecv(client, server, []byte("peer1"))
		tc.SendRecv(server, client, []byte("peer2"))
	}
}
