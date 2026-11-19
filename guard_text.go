package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// promptSegments maps the hash of every normalized sentence or line in a
// request to its text. Learned rules match on these hashes.
type promptSegments map[string]string

const maxSegmentTextRunes = 500

var segmentBreak = regexp.MustCompile(`[\r\n]+|[.!?。！？]+(?:\s+|$)`)

// requestSegments collects prompt text from every scope the official
// traversal knows (system, user, assistant, tool results, tool descriptions).
func requestSegments(body []byte) promptSegments {
	segments := promptSegments{}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return segments
	}
	collect := func(text string) (string, bool) {
		for _, segment := range splitSegments(text) {
			segments[segmentHash(segment)] = truncateRunes(segment, maxSegmentTextRunes)
		}
		return text, false
	}
	transforms := make(map[scopeBlock]promptTransform, len(scopeBlocks))
	for _, block := range scopeBlocks {
		transforms[block] = collect
	}
	transformPromptObject(root, transforms)
	return segments
}

// splitSegments breaks text into normalized sentences and lines.
func splitSegments(text string) []string {
	var out []string
	for _, part := range segmentBreak.Split(text, -1) {
		if segment := normalizeSegment(part); utf8.RuneCountInString(segment) >= 2 {
			out = append(out, segment)
		}
	}
	return out
}

func normalizeSegment(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func segmentHash(segment string) string {
	sum := sha256.Sum256([]byte(segment))
	return hex.EncodeToString(sum[:12])
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}

func (s promptSegments) hashes() []string {
	out := make([]string, 0, len(s))
	for hash := range s {
		out = append(out, hash)
	}
	return out
}
