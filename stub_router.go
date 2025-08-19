//go:build stub
// +build stub

package qzmq

import (
	"sync"
)

// stubRouter handles message routing between stub sockets
type stubRouter struct {
	mu         sync.RWMutex
	endpoints  map[string]*stubSocket
	bindings   map[string][]*stubSocket // endpoint -> sockets bound to it
	connections map[*stubSocket][]string // socket -> endpoints it's connected to
}

var globalRouter = &stubRouter{
	endpoints:   make(map[string]*stubSocket),
	bindings:    make(map[string][]*stubSocket),
	connections: make(map[*stubSocket][]string),
}

// registerBinding registers a socket as bound to an endpoint
func (r *stubRouter) registerBinding(endpoint string, socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.endpoints[endpoint] = socket
	r.bindings[endpoint] = append(r.bindings[endpoint], socket)
}

// registerConnection registers a socket as connected to an endpoint
func (r *stubRouter) registerConnection(endpoint string, socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.connections[socket] = append(r.connections[socket], endpoint)
}

// route routes a message from sender to appropriate receiver(s)
func (r *stubRouter) route(sender *stubSocket, data []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Find connected endpoints for this socket
	endpoints := r.connections[sender]
	
	for _, endpoint := range endpoints {
		// Find sockets bound to this endpoint
		if receivers, ok := r.bindings[endpoint]; ok {
			for _, receiver := range receivers {
				if receiver != sender {
					// Route message to receiver
					select {
					case receiver.inbound <- data:
						// Message delivered
					default:
						// Receiver queue full, drop message
					}
				}
			}
		}
	}
}

// routeMultipart routes a multipart message
func (r *stubRouter) routeMultipart(sender *stubSocket, parts [][]byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Find connected endpoints for this socket
	endpoints := r.connections[sender]
	
	for _, endpoint := range endpoints {
		// Find sockets bound to this endpoint
		if receivers, ok := r.bindings[endpoint]; ok {
			for _, receiver := range receivers {
				if receiver != sender {
					// Route multipart message to receiver
					select {
					case receiver.multipart <- parts:
						// Message delivered
					default:
						// Receiver queue full, drop message
					}
				}
			}
		}
	}
}

// findPeer finds a peer socket for REQ-REP pattern
func (r *stubRouter) findPeer(socket *stubSocket) *stubSocket {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// For REQ socket, find connected REP socket
	if socket.socketType == REQ {
		for _, endpoint := range r.connections[socket] {
			if receivers, ok := r.bindings[endpoint]; ok {
				for _, receiver := range receivers {
					if receiver.socketType == REP {
						return receiver
					}
				}
			}
		}
	}
	
	// For REP socket, find any connected REQ socket
	if socket.socketType == REP {
		for _, sockets := range r.bindings {
			for _, s := range sockets {
				if s == socket {
					// This is our binding, look for connected REQ sockets
					for conn, endpoints := range r.connections {
						for _, ep := range endpoints {
							if bound, ok := r.bindings[ep]; ok {
								for _, b := range bound {
									if b == socket && conn.socketType == REQ {
										return conn
									}
								}
							}
						}
					}
				}
			}
		}
	}
	
	return nil
}

// unregister removes a socket from all registrations
func (r *stubRouter) unregister(socket *stubSocket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Remove from connections
	delete(r.connections, socket)
	
	// Remove from bindings
	for endpoint, sockets := range r.bindings {
		var filtered []*stubSocket
		for _, s := range sockets {
			if s != socket {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(r.bindings, endpoint)
			delete(r.endpoints, endpoint)
		} else {
			r.bindings[endpoint] = filtered
		}
	}
}