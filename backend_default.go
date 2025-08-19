//go:build !czmq
// +build !czmq

package qzmq

import "errors"

// Default stub functions when CZMQ is not available

func initCZMQBackend() error {
	return errors.New("CZMQ backend not available - use Go backend")
}

func newCZMQSocket(socketType SocketType, opts Options) (Socket, error) {
	return nil, errors.New("CZMQ backend not available - use Go backend")
}
