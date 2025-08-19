//go:build !stub
// +build !stub

package qzmq

import (
	"context"
	"fmt"
	"sync"

	zmq "github.com/luxfi/zmq/v4"
)

// zmqSocket implements Socket using luxfi/zmq
// This automatically uses CZMQ when CGO=1 and czmq tag is set,
// otherwise falls back to pure Go implementation
type zmqSocket struct {
	socket     zmq.Socket
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	mu         sync.RWMutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Initialize the backend
func initGoBackend() error {
	// luxfi/zmq handles backend selection automatically
	return nil
}

// Create a new socket using luxfi/zmq
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	// Create context for the socket
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create the appropriate socket type using luxfi/zmq
	var socket zmq.Socket
	
	switch socketType {
	case REQ:
		socket = zmq.NewReq(ctx)
	case REP:
		socket = zmq.NewRep(ctx)
	case PUB:
		socket = zmq.NewPub(ctx)
	case SUB:
		socket = zmq.NewSub(ctx)
	case XPUB:
		socket = zmq.NewXPub(ctx)
	case XSUB:
		socket = zmq.NewXSub(ctx)
	case PUSH:
		socket = zmq.NewPush(ctx)
	case PULL:
		socket = zmq.NewPull(ctx)
	case PAIR:
		socket = zmq.NewPair(ctx)
	case DEALER:
		socket = zmq.NewDealer(ctx)
	case ROUTER:
		socket = zmq.NewRouter(ctx)
	default:
		cancel()
		return nil, fmt.Errorf("unsupported socket type: %v", socketType)
	}
	
	if socket == nil {
		cancel()
		return nil, fmt.Errorf("failed to create socket")
	}
	
	return &zmqSocket{
		socket:     socket,
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (s *zmqSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Listen(endpoint)
}

func (s *zmqSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Dial(endpoint)
}

func (s *zmqSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Create a message
	msg := zmq.NewMsg(data)
	
	// Send the message
	err := s.socket.Send(msg)
	if err != nil {
		return err
	}
	
	// Update metrics
	s.metrics.MessagesSent++
	s.metrics.BytesSent += uint64(len(data))
	
	return nil
}

func (s *zmqSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Create multi-part message
	msg := zmq.NewMsgFrom(parts...)
	
	// Send the message
	err := s.socket.Send(msg)
	if err != nil {
		return err
	}
	
	// Update metrics
	s.metrics.MessagesSent++
	for _, part := range parts {
		s.metrics.BytesSent += uint64(len(part))
	}
	
	return nil
}

func (s *zmqSocket) Recv() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	// Receive message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}
	
	// Get the first frame
	if len(msg.Frames) == 0 {
		return []byte{}, nil
	}
	
	data := msg.Frames[0]
	
	// Update metrics
	s.metrics.MessagesReceived++
	s.metrics.BytesReceived += uint64(len(data))
	
	return data, nil
}

func (s *zmqSocket) RecvMultipart() ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	// Receive message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}
	
	// Get all frames
	frames := msg.Frames
	
	// Update metrics
	s.metrics.MessagesReceived++
	for _, frame := range frames {
		s.metrics.BytesReceived += uint64(len(frame))
	}
	
	return frames, nil
}

func (s *zmqSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}
	
	// For XSUB, send subscription as a message
	if s.socketType == XSUB {
		// XSUB subscription format: first byte 1 means subscribe, followed by topic
		subMsg := append([]byte{1}, []byte(filter)...)
		msg := zmq.NewMsg(subMsg)
		return s.socket.Send(msg)
	}
	
	// For SUB, use SetOption
	return s.socket.SetOption(zmq.OptionSubscribe, filter)
}

func (s *zmqSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}
	
	// For XSUB, send unsubscription as a message
	if s.socketType == XSUB {
		// XSUB unsubscription format: first byte 0 means unsubscribe, followed by topic
		unsubMsg := append([]byte{0}, []byte(filter)...)
		msg := zmq.NewMsg(unsubMsg)
		return s.socket.Send(msg)
	}
	
	// For SUB, use SetOption
	return s.socket.SetOption(zmq.OptionUnsubscribe, filter)
}

func (s *zmqSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Map common options to ZMQ options
	switch name {
	case "sndhwm", "rcvhwm":
		if v, ok := value.(int); ok {
			return s.socket.SetOption(zmq.OptionHWM, v)
		}
	case "linger":
		// luxfi/zmq doesn't have linger option, ignore for now
		return nil
	case "identity":
		// luxfi/zmq handles identity differently
		return nil
	case "rcvtimeo", "sndtimeo":
		// Timeout options - ignore for now as luxfi/zmq handles timeouts differently
		return nil
	}
	
	return fmt.Errorf("unsupported option: %s", name)
}

func (s *zmqSocket) GetOption(name string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	switch name {
	case "type":
		return s.socketType, nil
	case "suite", "qzmq.suite":
		return s.opts.Suite, nil
	case "qzmq.encrypted":
		return s.opts.Suite.KEM != 0 || s.opts.Suite.Sign != 0, nil
	case "sndhwm", "rcvhwm":
		// luxfi/zmq doesn't expose these options directly
		return 1000, nil // default value
	case "identity":
		// luxfi/zmq handles identity differently
		return "", nil
	default:
		return nil, fmt.Errorf("unsupported option: %s", name)
	}
}

func (s *zmqSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	s.cancel() // Cancel context to close socket
	return s.socket.Close()
}

func (s *zmqSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}