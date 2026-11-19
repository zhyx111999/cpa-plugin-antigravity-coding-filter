package main

import (
	"sync"
	"time"
)

// pendingStore keeps the segments of requests forwarded to Google until the
// host reports how they ended. Entries the host never completes expire.
type pendingStore struct {
	mu      sync.Mutex
	entries map[string]pendingEntry
}

type pendingEntry struct {
	segments promptSegments
	model    string
	at       time.Time
}

const pendingTTL = 15 * time.Minute

var pendingRequests = &pendingStore{entries: map[string]pendingEntry{}}

func (p *pendingStore) put(requestID string, entry pendingEntry) {
	if requestID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, old := range p.entries {
		if entry.at.Sub(old.at) > pendingTTL {
			delete(p.entries, id)
		}
	}
	p.entries[requestID] = entry
}

func (p *pendingStore) take(requestID string) (pendingEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[requestID]
	delete(p.entries, requestID)
	return entry, ok
}
