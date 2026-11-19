package main

import (
	"encoding/json"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// guardComplete learns from how a forwarded request ended: a success clears
// its segments, Google's content-filter 429 turns its suspect segments into a rule.
func guardComplete(request []byte) []byte {
	var completion pluginapi.RequestCompletion
	if err := json.Unmarshal(request, &completion); err == nil {
		if entry, ok := pendingRequests.take(completion.RequestID); ok && activeGuardConfig().Enabled {
			switch {
			case completion.Outcome == pluginapi.RequestCompletionSucceeded:
				innocentSegments.add(entry.segments.hashes())
				learnedRules.prune(entry.segments)
			case isGoogleContentBlock(completion):
				suspects := innocentSegments.suspects(entry.segments.hashes())
				learnedRules.learn(entry.segments, suspects, entry.model, time.Now())
			}
		}
	}
	return mustEnvelope(struct{}{})
}
