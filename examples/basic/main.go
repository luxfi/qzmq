package main

import (
	"fmt"
	"log"
	"time"

	"github.com/luxfi/qzmq"
)

func main() {
	// Create QZMQ transport with hybrid post-quantum security
	opts := qzmq.DefaultOptions()
	opts.Suite = qzmq.Suite{
		KEM:  qzmq.HybridX25519MLKEM768,
		Sign: qzmq.MLDSA2,
		AEAD: qzmq.AES256GCM,
		Hash: qzmq.SHA256,
	}
	
	transport, err := qzmq.New(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer transport.Close()
	
	// Start server in goroutine
	go runServer(transport)
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Run client
	runClient(transport)
}

func runServer(transport qzmq.Transport) {
	// Create reply socket
	socket, err := transport.NewSocket(qzmq.REP)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()
	
	// Bind to endpoint
	if err := socket.Bind("tcp://*:5555"); err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("Server: Listening on tcp://*:5555 with QZMQ encryption")
	
	// Handle requests
	for i := 0; i < 3; i++ {
		// Receive encrypted message
		msg, err := socket.Recv()
		if err != nil {
			log.Printf("Server: Recv error: %v", err)
			continue
		}
		
		fmt.Printf("Server: Received: %s\n", msg)
		
		// Send encrypted reply
		reply := fmt.Sprintf("Hello from quantum-safe server! (msg %d)", i+1)
		if err := socket.Send([]byte(reply)); err != nil {
			log.Printf("Server: Send error: %v", err)
		}
	}
	
	// Print metrics
	metrics := socket.GetMetrics()
	fmt.Printf("\nServer Metrics:\n")
	fmt.Printf("  Messages sent: %d\n", metrics.MessagesSent)
	fmt.Printf("  Messages received: %d\n", metrics.MessagesReceived)
	fmt.Printf("  Bytes sent: %d\n", metrics.BytesSent)
	fmt.Printf("  Bytes received: %d\n", metrics.BytesReceived)
}

func runClient(transport qzmq.Transport) {
	// Create request socket
	socket, err := transport.NewSocket(qzmq.REQ)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()
	
	// Connect to server
	if err := socket.Connect("tcp://localhost:5555"); err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("\nClient: Connected to server with QZMQ encryption")
	
	// Send requests
	for i := 0; i < 3; i++ {
		// Send encrypted message
		msg := fmt.Sprintf("Hello from quantum-safe client! (msg %d)", i+1)
		if err := socket.Send([]byte(msg)); err != nil {
			log.Printf("Client: Send error: %v", err)
			continue
		}
		
		// Receive encrypted reply
		reply, err := socket.Recv()
		if err != nil {
			log.Printf("Client: Recv error: %v", err)
			continue
		}
		
		fmt.Printf("Client: Received: %s\n", reply)
	}
	
	// Print metrics
	metrics := socket.GetMetrics()
	fmt.Printf("\nClient Metrics:\n")
	fmt.Printf("  Messages sent: %d\n", metrics.MessagesSent)
	fmt.Printf("  Messages received: %d\n", metrics.MessagesReceived)
	fmt.Printf("  Bytes sent: %d\n", metrics.BytesSent)
	fmt.Printf("  Bytes received: %d\n", metrics.BytesReceived)
	
	// Print transport stats
	stats := transport.Stats()
	fmt.Printf("\nTransport Statistics:\n")
	fmt.Printf("  Handshakes completed: %d\n", stats.HandshakesCompleted)
	fmt.Printf("  Messages encrypted: %d\n", stats.MessagesEncrypted)
	fmt.Printf("  Messages decrypted: %d\n", stats.MessagesDecrypted)
	fmt.Printf("  Key rotations: %d\n", stats.KeyRotations)
}