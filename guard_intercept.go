package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// guardInterceptAfter wraps the official after-auth interceptor for
// Antigravity requests: rule-library hits (official keywords and the guard
// defaults) and learned-rule hits get the fixed message; forwarded requests
// are remembered for learning.
func guardInterceptAfter(request []byte) []byte {
	var req pluginapi.RequestInterceptRequest
	if err := json.Unmarshal(request, &req); err != nil || !strings.EqualFold(strings.TrimSpace(req.ToFormat), "antigravity") {
		return handleRequestInterceptAfter(request)
	}

	raw := handleRequestInterceptAfter(request)
	result, ok := decodeEnvelopeResult(raw)
	if !ok {
		return raw
	}
	var official pluginapi.RequestInterceptResponse
	if err := json.Unmarshal(result, &official); err != nil {
		return raw
	}
	body := req.Body
	if len(official.Body) > 0 {
		body = official.Body
	}
	if official.Terminate || guardRuleLibraryHit(body) {
		return mustEnvelope(guardBlockedResponse(req.SourceFormat, blockErrorCode))
	}
	now := time.Now()
	segments := requestSegments(body)
	if _, hit := learnedRules.match(segments, now); hit {
		return mustEnvelope(guardBlockedResponse(req.SourceFormat, guardLearnedCode))
	}
	pendingRequests.put(req.RequestID, pendingEntry{segments: segments, model: req.Model, at: now})
	return raw
}
