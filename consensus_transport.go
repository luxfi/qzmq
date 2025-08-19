// Package qzmq provides quantum-safe transport for Lux consensus
package qzmq

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// ConsensusTransport provides quantum-secure messaging for Lux validators
type ConsensusTransport struct {
	transport Transport
	nodeID    string
	
	// Validator keys (matches Lux consensus key hierarchy)
	blsKey    []byte // BLS key for classical finality
	pqKey     []byte // Lattice key for quantum finality
	
	// Consensus-specific sockets
	proposer  Socket // For proposing blocks (PUB)
	voter     Socket // For voting on proposals (SUB)
	gossip    Socket // For gossiping transactions (DEALER/ROUTER)
	sync      Socket // For state sync (REQ/REP)
	
	// Metrics
	metrics   *ConsensusMetrics
	mu        sync.RWMutex
}

// ConsensusMetrics tracks consensus-specific metrics
type ConsensusMetrics struct {
	ProposalsSent     uint64
	ProposalsReceived uint64
	VotesSent         uint64
	VotesReceived     uint64
	BlocksFinalized   uint64
	QuantumCerts      uint64
	ConsensusRounds   uint64
	AverageFinality   time.Duration
}

// ConsensusMessage represents a consensus protocol message
type ConsensusMessage struct {
	Type      MessageType
	ChainID   string
	Height    uint64
	Round     uint32
	Payload   []byte
	
	// Signatures (dual-signed for quantum security)
	BLSSig    []byte // Classical BLS signature
	PQSig     []byte // Post-quantum signature (ML-DSA)
	
	// For Quasar consensus
	Confidence float64 // Confidence level d(T)
	Phase      uint8   // 1=Propose, 2=Commit
}

// MessageType defines consensus message types
type MessageType uint8

const (
	// Core consensus messages
	MsgPropose MessageType = iota
	MsgVote
	MsgCommit
	MsgFinalize
	
	// Quasar-specific
	MsgSample      // k-peer sampling
	MsgConfidence  // Confidence building
	MsgThreshold   // Threshold reached
	MsgCertificate // Quantum certificate
	
	// Sync messages
	MsgSyncRequest
	MsgSyncResponse
	MsgGossip
)

// NewConsensusTransport creates a new consensus transport for a validator
func NewConsensusTransport(nodeID string, blsKey, pqKey []byte) (*ConsensusTransport, error) {
	// Create transport with Quasar-optimized settings
	opts := QuasarOptions()
	
	transport, err := New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	
	ct := &ConsensusTransport{
		transport: transport,
		nodeID:    nodeID,
		blsKey:    blsKey,
		pqKey:     pqKey,
		metrics:   &ConsensusMetrics{},
	}
	
	// Initialize consensus sockets
	if err := ct.initializeSockets(); err != nil {
		return nil, err
	}
	
	return ct, nil
}

// QuasarOptions returns optimized options for Quasar consensus
func QuasarOptions() Options {
	return Options{
		Backend: BackendAuto,
		Suite: Suite{
			// Use hybrid for transition period, then move to pure PQ
			KEM:  HybridX25519MLKEM768,  // Quantum-secure key exchange
			Sign: MLDSA3,                 // ML-DSA for signatures
			AEAD: AES256GCM,              // Fast symmetric encryption
			Hash: SHA384,                 // Strong hash for certificates
		},
		Mode: ModeHybrid, // Support both classical and PQ during transition
		
		// Aggressive key rotation for consensus
		KeyRotation: KeyRotationPolicy{
			MaxMessages: 10000,           // Rotate after 10K messages
			MaxBytes:    100 * 1024 * 1024, // Or 100MB
			MaxAge:      5 * time.Minute,   // Or 5 minutes
		},
		
		// Security features
		AntiDoS: true,  // Protect against DoS attacks
		ZeroRTT: false, // No 0-RTT for consensus (security > speed)
		
		// Consensus-optimized timeouts
		Timeouts: TimeoutConfig{
			Handshake: 500 * time.Millisecond, // Fast handshake for <1s finality
			Heartbeat: 5 * time.Second,        // Frequent heartbeats
			Linger:    0,                       // No lingering
		},
	}
}

func (ct *ConsensusTransport) initializeSockets() error {
	var err error
	
	// Proposer socket (broadcasts proposals)
	ct.proposer, err = ct.transport.NewSocket(PUB)
	if err != nil {
		return fmt.Errorf("failed to create proposer socket: %w", err)
	}
	
	// Voter socket (receives proposals)
	ct.voter, err = ct.transport.NewSocket(SUB)
	if err != nil {
		return fmt.Errorf("failed to create voter socket: %w", err)
	}
	ct.voter.Subscribe("") // Subscribe to all proposals
	
	// Gossip socket (peer-to-peer transaction sharing)
	ct.gossip, err = ct.transport.NewSocket(DEALER)
	if err != nil {
		return fmt.Errorf("failed to create gossip socket: %w", err)
	}
	ct.gossip.SetOption("identity", ct.nodeID)
	
	// Sync socket (state synchronization)
	ct.sync, err = ct.transport.NewSocket(REP)
	if err != nil {
		return fmt.Errorf("failed to create sync socket: %w", err)
	}
	
	return nil
}

// ProposeBlock broadcasts a block proposal with quantum-secure signatures
func (ct *ConsensusTransport) ProposeBlock(chainID string, height uint64, block []byte) error {
	msg := &ConsensusMessage{
		Type:    MsgPropose,
		ChainID: chainID,
		Height:  height,
		Round:   1,
		Payload: block,
		Phase:   1, // Propose phase
	}
	
	// Dual-sign with both BLS and PQ keys
	msg.BLSSig = ct.signBLS(msg.Hash())
	msg.PQSig = ct.signPQ(msg.Hash())
	
	data, err := msg.Marshal()
	if err != nil {
		return err
	}
	
	ct.metrics.ProposalsSent++
	return ct.proposer.Send(data)
}

// Vote sends a vote for a proposal with quantum certificates
func (ct *ConsensusTransport) Vote(proposal *ConsensusMessage, accept bool) error {
	vote := &ConsensusMessage{
		Type:       MsgVote,
		ChainID:    proposal.ChainID,
		Height:     proposal.Height,
		Round:      proposal.Round,
		Confidence: ct.calculateConfidence(proposal),
		Phase:      2, // Commit phase
	}
	
	if accept {
		vote.Payload = []byte("ACCEPT")
	} else {
		vote.Payload = []byte("REJECT")
	}
	
	// Dual-sign the vote
	vote.BLSSig = ct.signBLS(vote.Hash())
	vote.PQSig = ct.signPQ(vote.Hash())
	
	data, err := vote.Marshal()
	if err != nil {
		return err
	}
	
	ct.metrics.VotesSent++
	
	// Send to proposer via gossip network
	return ct.gossip.Send(data)
}

// ReceiveProposal waits for and receives a block proposal
func (ct *ConsensusTransport) ReceiveProposal() (*ConsensusMessage, error) {
	data, err := ct.voter.Recv()
	if err != nil {
		return nil, err
	}
	
	msg := &ConsensusMessage{}
	if err := msg.Unmarshal(data); err != nil {
		return nil, err
	}
	
	// Verify both signatures
	if !ct.verifyBLS(msg.Hash(), msg.BLSSig) {
		return nil, fmt.Errorf("invalid BLS signature")
	}
	
	if !ct.verifyPQ(msg.Hash(), msg.PQSig) {
		return nil, fmt.Errorf("invalid PQ signature")
	}
	
	ct.metrics.ProposalsReceived++
	return msg, nil
}

// QuantumFinalize creates a quantum certificate for finalized blocks
func (ct *ConsensusTransport) QuantumFinalize(block []byte, votes []*ConsensusMessage) ([]byte, error) {
	// Aggregate BLS signatures
	blsAgg := ct.aggregateBLS(votes)
	
	// Create lattice-based quantum certificate
	pqCert := ct.createQuantumCertificate(block, votes)
	
	// Bundle both certificates
	certBundle := &CertBundle{
		BLSAgg: blsAgg,
		PQCert: pqCert,
	}
	
	ct.metrics.QuantumCerts++
	ct.metrics.BlocksFinalized++
	
	return certBundle.Marshal()
}

// CertBundle contains both classical and quantum certificates
type CertBundle struct {
	BLSAgg []byte // 96B BLS aggregate signature
	PQCert []byte // ~3KB lattice certificate
}

// Helper methods for crypto operations (stubs for now)
func (ct *ConsensusTransport) signBLS(data []byte) []byte {
	// TODO: Implement BLS signing with ct.blsKey
	sig := make([]byte, 96)
	rand.Read(sig)
	return sig
}

func (ct *ConsensusTransport) signPQ(data []byte) []byte {
	// TODO: Implement ML-DSA signing with ct.pqKey
	sig := make([]byte, 3293) // ML-DSA-87 signature size
	rand.Read(sig)
	return sig
}

func (ct *ConsensusTransport) verifyBLS(data []byte, sig []byte) bool {
	// TODO: Implement BLS verification
	return len(sig) == 96
}

func (ct *ConsensusTransport) verifyPQ(data []byte, sig []byte) bool {
	// TODO: Implement ML-DSA verification
	return len(sig) == 3293
}

func (ct *ConsensusTransport) aggregateBLS(votes []*ConsensusMessage) []byte {
	// TODO: Implement BLS signature aggregation
	agg := make([]byte, 96)
	rand.Read(agg)
	return agg
}

func (ct *ConsensusTransport) createQuantumCertificate(block []byte, votes []*ConsensusMessage) []byte {
	// TODO: Implement lattice-based certificate creation
	cert := make([]byte, 3072)
	rand.Read(cert)
	return cert
}

func (ct *ConsensusTransport) calculateConfidence(msg *ConsensusMessage) float64 {
	// Simplified confidence calculation for Quasar consensus
	// Real implementation would track sampling rounds
	return 0.95
}

// Marshal/Unmarshal helpers
func (msg *ConsensusMessage) Hash() []byte {
	// Simple hash for demo - use proper crypto hash in production
	data := fmt.Sprintf("%d:%s:%d:%d:%s", 
		msg.Type, msg.ChainID, msg.Height, msg.Round, msg.Payload)
	return []byte(data)
}

func (msg *ConsensusMessage) Marshal() ([]byte, error) {
	// TODO: Implement proper serialization (protobuf/msgpack)
	return msg.Hash(), nil
}

func (msg *ConsensusMessage) Unmarshal(data []byte) error {
	// TODO: Implement proper deserialization
	msg.Payload = data
	return nil
}

func (cb *CertBundle) Marshal() ([]byte, error) {
	// Concatenate certificates
	result := make([]byte, len(cb.BLSAgg)+len(cb.PQCert))
	copy(result, cb.BLSAgg)
	copy(result[len(cb.BLSAgg):], cb.PQCert)
	return result, nil
}

// Connect establishes quantum-secure connections to peer validators
func (ct *ConsensusTransport) Connect(peers []string) error {
	for _, peer := range peers {
		// Connect voter to peer's proposer
		if err := ct.voter.Connect(fmt.Sprintf("tcp://%s:5000", peer)); err != nil {
			return err
		}
		
		// Connect gossip to peer's gossip router
		if err := ct.gossip.Connect(fmt.Sprintf("tcp://%s:5001", peer)); err != nil {
			return err
		}
	}
	return nil
}

// Listen starts listening for consensus messages
func (ct *ConsensusTransport) Listen(proposerPort, gossipPort, syncPort int) error {
	// Bind proposer for broadcasting
	if err := ct.proposer.Bind(fmt.Sprintf("tcp://*:%d", proposerPort)); err != nil {
		return err
	}
	
	// Create router for gossip (handles multiple dealers)
	router, err := ct.transport.NewSocket(ROUTER)
	if err != nil {
		return err
	}
	if err := router.Bind(fmt.Sprintf("tcp://*:%d", gossipPort)); err != nil {
		return err
	}
	
	// Bind sync server
	if err := ct.sync.Bind(fmt.Sprintf("tcp://*:%d", syncPort)); err != nil {
		return err
	}
	
	// Start gossip router handler
	go ct.handleGossip(router)
	
	return nil
}

func (ct *ConsensusTransport) handleGossip(router Socket) {
	for {
		parts, err := router.RecvMultipart()
		if err != nil {
			continue
		}
		
		// Route message to appropriate handler
		if len(parts) >= 2 {
			identity := parts[0]
			message := parts[1]
			
			// Process and potentially forward
			ct.processGossipMessage(identity, message)
		}
	}
}

func (ct *ConsensusTransport) processGossipMessage(identity, message []byte) {
	// Process consensus votes and other gossip
	msg := &ConsensusMessage{}
	if err := msg.Unmarshal(message); err != nil {
		return
	}
	
	switch msg.Type {
	case MsgVote:
		ct.metrics.VotesReceived++
		// Process vote...
	case MsgConfidence:
		// Update confidence metrics...
	case MsgCertificate:
		ct.metrics.QuantumCerts++
		// Process quantum certificate...
	}
}

// Close gracefully shuts down the consensus transport
func (ct *ConsensusTransport) Close() error {
	ct.proposer.Close()
	ct.voter.Close()
	ct.gossip.Close()
	ct.sync.Close()
	return ct.transport.Close()
}

// GetMetrics returns consensus metrics
func (ct *ConsensusTransport) GetMetrics() *ConsensusMetrics {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	metrics := *ct.metrics
	return &metrics
}