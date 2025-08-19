//go:build !czmq && !cgo
// +build !czmq,!cgo

package qzmq

import (
    "fmt"
    "testing"
)

func TestDebugDealerRouter(t *testing.T) {
    // Clear global state
    globalStubRouter.mu.Lock()
    globalStubRouter.routers = make(map[string]*stubSocket)
    globalStubRouter.dealers = make(map[string]*stubSocket)
    globalStubRouter.mu.Unlock()

    transport, _ := New(DefaultOptions())
    defer transport.Close()

    // Create and bind ROUTER
    router, _ := transport.NewSocket(ROUTER)
    router.Bind("tcp://127.0.0.1:15557")
    
    // Check if router is registered
    globalStubRouter.mu.RLock()
    fmt.Printf("Routers registered: %v\n", globalStubRouter.routers)
    globalStubRouter.mu.RUnlock()

    // Create and connect DEALER
    dealer, _ := transport.NewSocket(DEALER)
    dealer.Connect("tcp://127.0.0.1:15557")
    
    // Check if dealer can find router
    stubDealer := dealer.(*stubSocket)
    globalStubRouter.mu.RLock()
    foundRouter := findConnectedRouter(stubDealer)
    globalStubRouter.mu.RUnlock()
    
    if foundRouter == nil {
        t.Error("DEALER cannot find connected ROUTER")
    } else {
        t.Log("DEALER found ROUTER successfully")
    }
}
