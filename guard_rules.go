package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// learnedRule blocks any request that contains every one of its segments.
// Segments seen later in successful requests are pruned, so rules narrow
// toward the sentence Google actually flags.
type learnedRule struct {
	ID        string           `json:"id"`
	CreatedAt time.Time        `json:"created_at"`
	Model     string           `json:"model,omitempty"`
	Segments  []learnedSegment `json:"segments"`
	Hits      int              `json:"hits"`
	LastHitAt *time.Time       `json:"last_hit_at,omitempty"`
}

type learnedSegment struct {
	Hash string `json:"hash"`
	Text string `json:"text"`
}

type learnedRuleFile struct {
	Rules []learnedRule `json:"rules"`
}

type ruleStore struct {
	mu    sync.Mutex
	path  string
	rules []learnedRule
}

var learnedRules = &ruleStore{}

// readLearnedRules reads a rules file. A missing file means no rules yet.
func readLearnedRules(path string) ([]learnedRule, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read learned rules: %w", err)
	}
	var file learnedRuleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode learned rules %s: %w", path, err)
	}
	return file.Rules, nil
}

// reset points the store at a file and replaces the in-memory rules.
func (s *ruleStore) reset(path string, rules []learnedRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = path
	s.rules = rules
}

// match returns the first rule whose segments are all present and records the hit.
func (s *ruleStore) match(segments promptSegments, now time.Time) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		if !ruleCovered(s.rules[i], segments) {
			continue
		}
		s.rules[i].Hits++
		hitAt := now.UTC()
		s.rules[i].LastHitAt = &hitAt
		s.saveLocked()
		return s.rules[i].ID, true
	}
	return "", false
}

// learn adds a rule from the suspect segments of a request Google rejected.
// An identical existing rule is kept instead of adding a duplicate.
func (s *ruleStore) learn(segments promptSegments, suspects []string, model string, now time.Time) string {
	if len(suspects) == 0 {
		suspects = segments.hashes()
	}
	if len(suspects) == 0 {
		return ""
	}
	sort.Strings(suspects)
	rule := learnedRule{ID: "r-" + segmentHash(strings.Join(suspects, ","))[:12], CreatedAt: now.UTC(), Model: model}
	for _, hash := range suspects {
		rule.Segments = append(rule.Segments, learnedSegment{Hash: hash, Text: segments[hash]})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.rules {
		if existing.ID == rule.ID {
			return existing.ID
		}
	}
	s.rules = append(s.rules, rule)
	s.saveLocked()
	return rule.ID
}

// prune removes segments that appeared in a successful request. A rule whose
// segments all appeared there was wrong and is dropped.
func (s *ruleStore) prune(succeeded promptSegments) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	kept := s.rules[:0]
	for _, rule := range s.rules {
		remaining := rule.Segments[:0:0]
		for _, segment := range rule.Segments {
			if _, seen := succeeded[segment.Hash]; !seen {
				remaining = append(remaining, segment)
			}
		}
		if len(remaining) != len(rule.Segments) {
			changed = true
		}
		if len(remaining) == 0 {
			continue
		}
		rule.Segments = remaining
		kept = append(kept, rule)
	}
	s.rules = kept
	if changed {
		s.saveLocked()
	}
}

func ruleCovered(rule learnedRule, segments promptSegments) bool {
	if len(rule.Segments) == 0 {
		return false
	}
	for _, segment := range rule.Segments {
		if _, ok := segments[segment.Hash]; !ok {
			return false
		}
	}
	return true
}

// saveLocked writes the rules atomically. Failures leave the previous file intact;
// the in-memory rules stay authoritative until the next successful write.
func (s *ruleStore) saveLocked() {
	if s.path == "" {
		return
	}
	raw, err := json.MarshalIndent(learnedRuleFile{Rules: s.rules}, "", "  ")
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".learned-rules-*")
	if err != nil {
		return
	}
	_, errWrite := tmp.Write(append(raw, '\n'))
	errClose := tmp.Close()
	if errWrite != nil || errClose != nil || os.Rename(tmp.Name(), s.path) != nil {
		_ = os.Remove(tmp.Name())
	}
}
