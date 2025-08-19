//go:build stub
// +build stub

package qzmq

import (
	"sync"
)

// stubPubSubBus manages PUB-SUB message routing
type stubPubSubBus struct {
	mu          sync.RWMutex
	subscribers map[string][]*stubSocket // topic -> subscribers
	channels    map[*stubSocket]chan []byte // socket -> channel for messages
}

var globalPubSubBus = &stubPubSubBus{
	subscribers: make(map[string][]*stubSocket),
	channels:    make(map[*stubSocket]chan []byte),
}

// subscribe adds a subscriber for a topic
func (b *stubPubSubBus) subscribe(socket *stubSocket, filter string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Ensure socket has a channel
	if _, ok := b.channels[socket]; !ok {
		b.channels[socket] = make(chan []byte, 1000)
	}
	
	// Add to subscribers for this filter
	b.subscribers[filter] = append(b.subscribers[filter], socket)
}

// unsubscribe removes a subscriber from a topic
func (b *stubPubSubBus) unsubscribe(socket *stubSocket, filter string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Remove from subscribers
	if subs, ok := b.subscribers[filter]; ok {
		var filtered []*stubSocket
		for _, s := range subs {
			if s != socket {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(b.subscribers, filter)
		} else {
			b.subscribers[filter] = filtered
		}
	}
}

// publish sends a message to all matching subscribers
func (b *stubPubSubBus) publish(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	// Check all filters
	for filter, subs := range b.subscribers {
		// Simple filter matching (empty filter = subscribe all)
		if filter == "" || len(data) >= len(filter) && string(data[:len(filter)]) == filter {
			// Send to all subscribers of this filter
			for _, sub := range subs {
				if ch, ok := b.channels[sub]; ok {
					select {
					case ch <- data:
						// Message sent
					default:
						// Channel full, drop message
					}
				}
			}
		}
	}
}

// getChannel returns the channel for a socket
func (b *stubPubSubBus) getChannel(socket *stubSocket) chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if ch, ok := b.channels[socket]; ok {
		return ch
	}
	
	// Create new channel
	ch := make(chan []byte, 1000)
	b.channels[socket] = ch
	return ch
}

// unregister removes a socket completely
func (b *stubPubSubBus) unregister(socket *stubSocket) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Remove from all subscriptions
	for filter, subs := range b.subscribers {
		var filtered []*stubSocket
		for _, s := range subs {
			if s != socket {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(b.subscribers, filter)
		} else {
			b.subscribers[filter] = filtered
		}
	}
	
	// Close and remove channel
	if ch, ok := b.channels[socket]; ok {
		close(ch)
		delete(b.channels, socket)
	}
}

// Global function for backward compatibility
func getSharedPubSubChannel() chan []byte {
	// This is a legacy function, return nil to use new system
	return nil
}