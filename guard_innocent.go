package main

import "sync"

// innocentSet remembers segment hashes that reached Google and succeeded.
// It keeps two generations so memory stays bounded without per-entry
// bookkeeping: when the current generation is full it becomes the previous one.
type innocentSet struct {
	mu       sync.Mutex
	limit    int
	current  map[string]struct{}
	previous map[string]struct{}
}

const innocentSetLimit = 200_000

var innocentSegments = newInnocentSet(innocentSetLimit)

func newInnocentSet(limit int) *innocentSet {
	return &innocentSet{limit: limit, current: map[string]struct{}{}, previous: map[string]struct{}{}}
}

func (s *innocentSet) add(hashes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, hash := range hashes {
		if len(s.current) >= s.limit {
			s.previous, s.current = s.current, make(map[string]struct{}, s.limit)
		}
		s.current[hash] = struct{}{}
	}
}

// suspects returns the hashes that have not been seen in a successful request.
func (s *innocentSet) suspects(hashes []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, hash := range hashes {
		_, inCurrent := s.current[hash]
		_, inPrevious := s.previous[hash]
		if !inCurrent && !inPrevious {
			out = append(out, hash)
		}
	}
	return out
}
