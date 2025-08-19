//go:build !czmq && !cgo
// +build !czmq,!cgo

package qzmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	zmq "github.com/luxfi/zmq/v4"
)

// pureSocket implements Socket using pure Go luxfi/zmq
type pureSocket struct {
	socket     zmq.Socket
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	mu         sync.RWMutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Initialize the pure Go backend
func initGoBackend() error {
	// Pure Go backend doesn't need initialization
	return nil
}

// Create a new socket using pure Go ZMQ
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	// Create context for the socket
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create the appropriate socket type
	var socket zmq.Socket
	var err error
	
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
	
	return &pureSocket{
		socket:     socket,
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (s *pureSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Listen(endpoint)
}

func (s *pureSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	return s.socket.Dial(endpoint)
}

func (s *pureSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Create a message
	msg := zmq.NewMsgFrom(data)
	
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

func (s *pureSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Create multi-part message
	frames := make([][]byte, len(parts))
	copy(frames, parts)
	msg := zmq.NewMsgFromFrames(frames)
	
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

func (s *pureSocket) Recv() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	// Receive message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}
	
	// Get the first frame
	frames := msg.Frames()
	if len(frames) == 0 {
		return []byte{}, nil
	}
	
	data := frames[0]
	
	// Update metrics
	s.metrics.MessagesReceived++
	s.metrics.BytesReceived += uint64(len(data))
	
	return data, nil
}

func (s *pureSocket) RecvMultipart() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, ErrNotConnected
	}
	
	// Receive message
	msg, err := s.socket.Recv()
	if err != nil {
		return nil, err
	}
	
	// Get all frames
	frames := msg.Frames()
	
	// Update metrics
	s.metrics.MessagesReceived++
	for _, frame := range frames {
		s.metrics.BytesReceived += uint64(len(frame))
	}
	
	return frames, nil
}

func (s *pureSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}
	
	// Type assert to get the subscriber interface
	if sub, ok := s.socket.(*zmq.Sub); ok {
		return sub.SetOption(zmq.OptionSubscribe, filter)
	} else if xsub, ok := s.socket.(*zmq.XSub); ok {
		return xsub.SetOption(zmq.OptionSubscribe, filter)
	}
	
	return fmt.Errorf("socket does not support subscriptions")
}

func (s *pureSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}
	
	// Type assert to get the subscriber interface
	if sub, ok := s.socket.(*zmq.Sub); ok {
		return sub.SetOption(zmq.OptionUnsubscribe, filter)
	} else if xsub, ok := s.socket.(*zmq.XSub); ok {
		return xsub.SetOption(zmq.OptionUnsubscribe, filter)
	}
	
	return fmt.Errorf("socket does not support unsubscribe")
}

func (s *pureSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return ErrNotConnected
	}
	
	// Map common options to ZMQ options
	switch name {
	case "sndhwm":
		if v, ok := value.(int); ok {
			return s.socket.SetOption(zmq.OptionSndHWM, v)
		}
	case "rcvhwm":
		if v, ok := value.(int); ok {
			return s.socket.SetOption(zmq.OptionRcvHWM, v)
		}
	case "linger":
		if v, ok := value.(int); ok {
			return s.socket.SetOption(zmq.OptionLinger, time.Duration(v)*time.Millisecond)
		}
	case "identity":
		if v, ok := value.(string); ok {
			return s.socket.SetOption(zmq.OptionIdentity, v)
		}
	}
	
	return fmt.Errorf("unsupported option: %s", name)
}

func (s *pureSocket) GetOption(name string) (interface{}, error) {
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
	case "sndhwm":
		return s.socket.GetOption(zmq.OptionSndHWM)
	case "rcvhwm":
		return s.socket.GetOption(zmq.OptionRcvHWM)
	case "identity":
		return s.socket.GetOption(zmq.OptionIdentity)
	default:
		return nil, fmt.Errorf("unsupported option: %s", name)
	}
}

func (s *pureSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	s.cancel() // Cancel context to close socket
	return s.socket.Close()
}

func (s *pureSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}

// Stubs for CZMQ backend when building pure Go
func initCZMQBackend() error {
	// Not available in pure Go build
	return fmt.Errorf("CZMQ backend not available in pure Go build")
}

func newCZMQSocket(socketType SocketType, opts Options) (Socket, error) {
	// Fall back to pure Go implementation
	return newGoSocket(socketType, opts)
}