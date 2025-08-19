//go:build czmq && cgo
// +build czmq,cgo

package qzmq

// #cgo LDFLAGS: -lczmq -lzmq
// #include <czmq.h>
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

// czmqSocket implements Socket using CZMQ (high-level C binding)
type czmqSocket struct {
	socket     unsafe.Pointer // zsock_t*
	socketType SocketType
	opts       Options
	metrics    *SocketMetrics
	mu         sync.RWMutex
	closed     bool
}

// Initialize the CZMQ backend
func initGoBackend() error {
	// CZMQ doesn't need explicit initialization
	return nil
}

// Create a new CZMQ socket
func newGoSocket(socketType SocketType, opts Options) (Socket, error) {
	// Map our socket types to CZMQ types
	var czmqType C.int
	switch socketType {
	case REQ:
		czmqType = C.ZMQ_REQ
	case REP:
		czmqType = C.ZMQ_REP
	case PUB:
		czmqType = C.ZMQ_PUB
	case SUB:
		czmqType = C.ZMQ_SUB
	case XPUB:
		czmqType = C.ZMQ_XPUB
	case XSUB:
		czmqType = C.ZMQ_XSUB
	case PUSH:
		czmqType = C.ZMQ_PUSH
	case PULL:
		czmqType = C.ZMQ_PULL
	case PAIR:
		czmqType = C.ZMQ_PAIR
	case DEALER:
		czmqType = C.ZMQ_DEALER
	case ROUTER:
		czmqType = C.ZMQ_ROUTER
	case STREAM:
		czmqType = C.ZMQ_STREAM
	default:
		return nil, fmt.Errorf("unsupported socket type: %v", socketType)
	}

	// Create the CZMQ socket
	socket := C.zsock_new(czmqType)
	if socket == nil {
		return nil, fmt.Errorf("failed to create CZMQ socket")
	}

	return &czmqSocket{
		socket:     unsafe.Pointer(socket),
		socketType: socketType,
		opts:       opts,
		metrics:    NewSocketMetrics(),
	}, nil
}

func (s *czmqSocket) Bind(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	cEndpoint := C.CString(endpoint)
	defer C.free(unsafe.Pointer(cEndpoint))

	rc := C.zsock_bind((*C.zsock_t)(s.socket), cEndpoint)
	if rc == -1 {
		return fmt.Errorf("bind failed")
	}
	return nil
}

func (s *czmqSocket) Connect(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	cEndpoint := C.CString(endpoint)
	defer C.free(unsafe.Pointer(cEndpoint))

	rc := C.zsock_connect((*C.zsock_t)(s.socket), cEndpoint)
	if rc == -1 {
		return fmt.Errorf("connect failed")
	}
	return nil
}

func (s *czmqSocket) Send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Create a CZMQ frame
	frame := C.zframe_new(unsafe.Pointer(&data[0]), C.size_t(len(data)))
	if frame == nil {
		return fmt.Errorf("failed to create frame")
	}

	// Send the frame (takes ownership)
	rc := C.zframe_send(&frame, (*C.zsock_t)(s.socket), 0)
	if rc == -1 {
		C.zframe_destroy(&frame)
		return fmt.Errorf("send failed")
	}

	// Update metrics
	s.metrics.MessagesSent++
	s.metrics.BytesSent += uint64(len(data))

	return nil
}

func (s *czmqSocket) SendMultipart(parts [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Create a CZMQ message
	msg := C.zmsg_new()
	if msg == nil {
		return fmt.Errorf("failed to create message")
	}

	// Add all parts to the message
	for _, part := range parts {
		frame := C.zframe_new(unsafe.Pointer(&part[0]), C.size_t(len(part)))
		if frame == nil {
			C.zmsg_destroy(&msg)
			return fmt.Errorf("failed to create frame")
		}
		C.zmsg_append(msg, &frame)
		s.metrics.BytesSent += uint64(len(part))
	}

	// Send the message (takes ownership)
	rc := C.zmsg_send(&msg, (*C.zsock_t)(s.socket))
	if rc == -1 {
		C.zmsg_destroy(&msg)
		return fmt.Errorf("send multipart failed")
	}

	s.metrics.MessagesSent++
	return nil
}

func (s *czmqSocket) Recv() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrNotConnected
	}

	// Receive a frame
	frame := C.zframe_recv((*C.zsock_t)(s.socket))
	if frame == nil {
		return nil, fmt.Errorf("receive failed")
	}
	defer C.zframe_destroy(&frame)

	// Get the data
	size := C.zframe_size(frame)
	data := C.GoBytes(unsafe.Pointer(C.zframe_data(frame)), C.int(size))

	// Update metrics
	s.metrics.MessagesReceived++
	s.metrics.BytesReceived += uint64(len(data))

	return data, nil
}

func (s *czmqSocket) RecvMultipart() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrNotConnected
	}

	// Receive a message
	msg := C.zmsg_recv((*C.zsock_t)(s.socket))
	if msg == nil {
		return nil, fmt.Errorf("receive multipart failed")
	}
	defer C.zmsg_destroy(&msg)

	// Extract all frames
	var parts [][]byte
	for {
		frame := C.zmsg_pop(msg)
		if frame == nil {
			break
		}
		size := C.zframe_size(frame)
		data := C.GoBytes(unsafe.Pointer(C.zframe_data(frame)), C.int(size))
		parts = append(parts, data)
		s.metrics.BytesReceived += uint64(len(data))
		C.zframe_destroy(&frame)
	}

	s.metrics.MessagesReceived++
	return parts, nil
}

func (s *czmqSocket) Subscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("subscribe only valid for SUB/XSUB sockets")
	}

	cFilter := C.CString(filter)
	defer C.free(unsafe.Pointer(cFilter))

	C.zsock_set_subscribe((*C.zsock_t)(s.socket), cFilter)
	return nil
}

func (s *czmqSocket) Unsubscribe(filter string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	if s.socketType != SUB && s.socketType != XSUB {
		return fmt.Errorf("unsubscribe only valid for SUB/XSUB sockets")
	}

	cFilter := C.CString(filter)
	defer C.free(unsafe.Pointer(cFilter))

	C.zsock_set_unsubscribe((*C.zsock_t)(s.socket), cFilter)
	return nil
}

func (s *czmqSocket) SetOption(name string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrNotConnected
	}

	// Map common options to CZMQ
	switch name {
	case "sndhwm":
		if v, ok := value.(int); ok {
			C.zsock_set_sndhwm((*C.zsock_t)(s.socket), C.int(v))
		}
	case "rcvhwm":
		if v, ok := value.(int); ok {
			C.zsock_set_rcvhwm((*C.zsock_t)(s.socket), C.int(v))
		}
	case "linger":
		if v, ok := value.(int); ok {
			C.zsock_set_linger((*C.zsock_t)(s.socket), C.int(v))
		}
	case "identity":
		if v, ok := value.(string); ok {
			cIdentity := C.CString(v)
			defer C.free(unsafe.Pointer(cIdentity))
			C.zsock_set_identity((*C.zsock_t)(s.socket), cIdentity)
		}
	}

	return nil
}

func (s *czmqSocket) GetOption(name string) (interface{}, error) {
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
		return int(C.zsock_sndhwm((*C.zsock_t)(s.socket))), nil
	case "rcvhwm":
		return int(C.zsock_rcvhwm((*C.zsock_t)(s.socket))), nil
	case "identity":
		identity := C.zsock_identity((*C.zsock_t)(s.socket))
		return C.GoString(identity), nil
	default:
		return nil, fmt.Errorf("unsupported option: %s", name)
	}
}

func (s *czmqSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	C.zsock_destroy((**C.zsock_t)(unsafe.Pointer(&s.socket)))
	return nil
}

func (s *czmqSocket) GetMetrics() *SocketMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}