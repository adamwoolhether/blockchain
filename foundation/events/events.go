// Package events allows for the registering and receiving of events.
package events

import (
	"fmt"
	"sync"
)

// Events maintains a mapping of unique id and channels
// so goroutines can register and received events.
type Events struct {
	m  map[string]chan string
	mu sync.RWMutex
}

// New constructs an events for registering and receiving events.
func New() *Events {
	return &Events{
		m: make(map[string]chan string),
	}
}

// Shutdown closes and removes all channels that were
// provided by the call to Acquire.
func (evt *Events) Shutdown() {
	evt.mu.RLock()
	defer evt.mu.RUnlock()
	
	for id, ch := range evt.m {
		delete(evt.m, id)
		close(ch)
	}
}

// Acquire takes a unique id and returns a channel that can
// be used to receive events.
func (evt *Events) Acquire(id string) chan string {
	evt.mu.RLock()
	defer evt.mu.RUnlock()
	
	ch, exists := evt.m[id]
	if exists {
		return ch
	}
	
	evt.m[id] = make(chan string)
	
	return evt.m[id]
}

// Release closes and removes the channel that was
// provided by the call to Acquire.
func (evt *Events) Release(id string) error {
	evt.mu.RLock()
	defer evt.mu.RUnlock()
	
	ch, exists := evt.m[id]
	if !exists {
		return fmt.Errorf("id %q does not exist", id)
	}
	
	delete(evt.m, id)
	close(ch)
	
	return nil
}

// Send signals a message to a registered channel. Send will not
// block waiting for a receiver on any given channel.
func (evt *Events) Send(s string) {
	evt.mu.RLock()
	defer evt.mu.RUnlock()
	
	for _, ch := range evt.m {
		select {
		case ch <- s:
		default:
		}
	}
}
