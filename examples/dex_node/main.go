// Example DEX node using QZMQ with X-Chain integration
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luxfi/qzmq"
)

func main() {
	// Load validator keys (these would come from luxfi/node key management)
	validatorKeys := &qzmq.ValidatorKeys{
		NodeID:  loadKey("$HOME/.lux/node.key"),       // Ed25519 for P2P
		BLSKey:  loadKey("$HOME/.lux/bls.key"),        // BLS12-381 for consensus
		PQKey:   loadKey("$HOME/.lux/rt.key"),         // ML-KEM-768 for PQ security
		DSAKey:  loadKey("$HOME/.lux/mldsa.key"),      // ML-DSA-87 for signatures
	}
	
	// Create X-Chain DEX transport with quantum security
	dexTransport, err := qzmq.NewXChainDEXTransport(
		"http://localhost:9650",  // Lux node RPC
		"X",                      // X-Chain
		"NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg", // Validator ID
		validatorKeys,
	)
	if err != nil {
		log.Fatalf("Failed to create DEX transport: %v", err)
	}
	defer dexTransport.Close()
	
	// Start DEX services
	// Ports align with Lux node conventions:
	// - Consensus: 5000-5002 (Quasar consensus)
	// - DEX: 6000-6003 (X-Chain DEX)
	// - P2P: 9651 (standard Lux P2P)
	
	// Start consensus layer (integrates with Quasar)
	if err := dexTransport.Listen(5000, 5001, 5002); err != nil {
		log.Fatalf("Failed to start consensus: %v", err)
	}
	
	// Start DEX services
	if err := dexTransport.StartDEX(6000, 6001, 6002, 6003); err != nil {
		log.Fatalf("Failed to start DEX: %v", err)
	}
	
	// Connect to peer validators
	peers := []string{
		"192.168.1.2", // Validator 2
		"192.168.1.3", // Validator 3
		// Add more validators as needed
	}
	
	if err := dexTransport.ConnectToPeers(peers); err != nil {
		log.Fatalf("Failed to connect to peers: %v", err)
	}
	
	log.Println("X-Chain DEX node started with quantum security")
	log.Println("Services:")
	log.Println("  - Consensus (Quasar): ports 5000-5002")
	log.Println("  - Order Book: port 6000")
	log.Println("  - Matching Engine: port 6001")
	log.Println("  - Settlement: port 6002")
	log.Println("  - Market Data: port 6003")
	
	// Example: Submit an order with quantum signature
	go func() {
		time.Sleep(5 * time.Second)
		
		order := &qzmq.Order{
			ID:       "order-1",
			Market:   "LUX-USDC",
			Side:     qzmq.Buy,
			Type:     qzmq.Limit,
			Price:    1000000000, // $10.00 with 8 decimals
			Quantity: 100000000,  // 1.00 LUX
			UserAddr: "X-lux1q0qwerty...",
			AssetID:  "2mWqzgywhqCgWwXzeBJoZqVAxDJb3hjbE3MbCHdkdYSSyPpNkV", // LUX asset ID
		}
		
		// Sign order with ML-DSA (quantum-secure signature)
		order.Signature = signOrderWithMLDSA(order, validatorKeys.DSAKey)
		order.Certificate = generateQuantumCertificate(order)
		
		if err := dexTransport.SubmitOrder(order); err != nil {
			log.Printf("Failed to submit order: %v", err)
		} else {
			log.Printf("Order submitted: %s", order.ID)
		}
	}()
	
	// Example: Participate in consensus
	go func() {
		for {
			// Receive block proposals
			proposal, err := dexTransport.ReceiveProposal()
			if err != nil {
				continue
			}
			
			log.Printf("Received proposal for height %d", proposal.Height)
			
			// Vote on proposal (this integrates with Quasar consensus)
			if err := dexTransport.Vote(proposal, true); err != nil {
				log.Printf("Failed to vote: %v", err)
			}
		}
	}()
	
	// Monitor metrics
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			metrics := dexTransport.GetMetrics()
			log.Printf("Consensus Metrics:")
			log.Printf("  Proposals: sent=%d received=%d", 
				metrics.ProposalsSent, metrics.ProposalsReceived)
			log.Printf("  Votes: sent=%d received=%d",
				metrics.VotesSent, metrics.VotesReceived)
			log.Printf("  Blocks finalized: %d", metrics.BlocksFinalized)
			log.Printf("  Quantum certificates: %d", metrics.QuantumCerts)
			
			// Also get QZMQ transport metrics
			stats := dexTransport.Stats()
			log.Printf("Transport Metrics:")
			log.Printf("  Messages: encrypted=%d decrypted=%d",
				stats.MessagesEncrypted, stats.MessagesDecrypted)
			log.Printf("  Key rotations: %d", stats.KeyRotations)
			log.Printf("  Auth failures: %d", stats.AuthFailures)
		}
	}()
	
	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	log.Println("Shutting down DEX node...")
}

// Helper functions (these would use actual luxfi/crypto in production)
func loadKey(path string) []byte {
	// In production, this would load the actual key from disk
	// For demo, return dummy key
	switch path {
	case "$HOME/.lux/node.key":
		return make([]byte, 32) // Ed25519
	case "$HOME/.lux/bls.key":
		return make([]byte, 32) // BLS secret key
	case "$HOME/.lux/rt.key":
		return make([]byte, 2400) // ML-KEM-768 secret key
	case "$HOME/.lux/mldsa.key":
		return make([]byte, 4032) // ML-DSA-87 secret key
	default:
		return nil
	}
}

func signOrderWithMLDSA(order *qzmq.Order, key []byte) []byte {
	// In production, use luxfi/crypto ML-DSA implementation
	// For demo, return dummy signature
	return make([]byte, 3293) // ML-DSA-87 signature size
}

func generateQuantumCertificate(order *qzmq.Order) []byte {
	// Generate quantum certificate for order validity
	// This would use luxfi/crypto lattice-based crypto
	return make([]byte, 3072)
}

// Integration with luxfi/node
// This shows how QZMQ integrates with the existing Lux node infrastructure
func integrateWithLuxNode() {
	// 1. QZMQ replaces standard ZeroMQ in network layer
	// 2. Consensus messages use quantum signatures
	// 3. DEX operations are quantum-secure end-to-end
	// 4. Key management integrates with existing Lux key store
	
	fmt.Println(`
Integration Points:
===================

1. luxfi/crypto provides:
   - ML-KEM-768/1024 for key exchange
   - ML-DSA-87 for signatures  
   - BLS12-381 for aggregation
   - Ed25519 for node identity

2. luxfi/qzmq provides:
   - Quantum-secure transport layer
   - Automatic key rotation
   - Post-quantum handshakes
   - AEAD encryption with MLKEM-derived keys

3. luxfi/consensus uses:
   - QZMQ for validator communication
   - Dual signatures (BLS + ML-DSA)
   - Quantum certificates for finality
   - Secure gossip protocol

4. luxfi/node orchestrates:
   - Key loading and management
   - Service initialization
   - Peer discovery and connection
   - RPC and API endpoints

5. X-Chain DEX benefits:
   - Quantum-secure order submission
   - Protected market data feeds
   - Secure settlement proofs
   - Future-proof against quantum attacks
`)
}