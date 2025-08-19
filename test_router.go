//go:build stub
// +build stub

package qzmq

import (
	"sync"
	"strings"
)

// testRouter provides simple message routing for testing with stub backend
type testRouter struct {
	mu       sync.RWMutex
	channels map[string]chan []byte
	patterns map[SocketType]func(*stubSocket, []byte) []byte
}

var globalRouter = &testRouter{
	channels: make(map[string]chan []byte),
	patterns: make(map[SocketType]func(*stubSocket, []byte) []byte),
}

func init() {
	// Set up pattern-specific behavior
	globalRouter.patterns[REQ] = reqPattern
	globalRouter.patterns[REP] = repPattern
	globalRouter.patterns[PUB] = pubPattern
	globalRouter.patterns[SUB] = subPattern
	globalRouter.patterns[PUSH] = pushPattern
	globalRouter.patterns[PULL] = pullPattern
	globalRouter.patterns[DEALER] = dealerPattern
	globalRouter.patterns[ROUTER] = routerPattern
	globalRouter.patterns[PAIR] = pairPattern
}

// reqPattern handles REQ socket behavior
func reqPattern(s *stubSocket, msg []byte) []byte {
	// REQ expects to send a request and get a reply
	// For testing, create expected response
	msgStr := string(msg)
	if strings.Contains(msgStr, "Request") {
		// Generate corresponding response
		return []byte(strings.Replace(msgStr, "Request", "Response", 1))
	}
	if msgStr == "Hello QZMQ" {
		return []byte("Hello Client")
	}
	// Default echo with prefix
	return append([]byte("Reply: "), msg...)
}

// repPattern handles REP socket behavior
func repPattern(s *stubSocket, msg []byte) []byte {
	// REP receives requests and sends replies
	// This is typically the server side, so just echo
	return msg
}

// pubPattern handles PUB socket behavior
func pubPattern(s *stubSocket, msg []byte) []byte {
	// PUB broadcasts to all subscribers
	return msg
}

// subPattern handles SUB socket behavior
func subPattern(s *stubSocket, msg []byte) []byte {
	// SUB receives published messages
	return msg
}

// pushPattern handles PUSH socket behavior
func pushPattern(s *stubSocket, msg []byte) []byte {
	// PUSH sends to pullers
	return msg
}

// pullPattern handles PULL socket behavior
func pullPattern(s *stubSocket, msg []byte) []byte {
	// PULL receives from pushers
	return msg
}

// dealerPattern handles DEALER socket behavior
func dealerPattern(s *stubSocket, msg []byte) []byte {
	// DEALER can send/receive freely
	// For testing, echo back
	return msg
}

// routerPattern handles ROUTER socket behavior
func routerPattern(s *stubSocket, msg []byte) []byte {
	// ROUTER can route to specific dealers
	// For testing, echo back
	return msg
}

// pairPattern handles PAIR socket behavior
func pairPattern(s *stubSocket, msg []byte) []byte {
	// PAIR is bidirectional
	return msg
}

// getConnectedChannel finds or creates a channel for connected sockets
func getConnectedChannel(endpoint string) chan []byte {
	globalRouter.mu.Lock()
	defer globalRouter.mu.Unlock()
	
	if ch, ok := globalRouter.channels[endpoint]; ok {
		return ch
	}
	
	ch := make(chan []byte, 100)
	globalRouter.channels[endpoint] = ch
	return ch
}

// routeMessage routes a message based on socket type
func routeMessage(s *stubSocket, msg []byte) []byte {
	if pattern, ok := globalRouter.patterns[s.socketType]; ok {
		return pattern(s, msg)
	}
	return msg
}