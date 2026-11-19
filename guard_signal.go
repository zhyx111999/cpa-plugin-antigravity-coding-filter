package main

import (
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// isGoogleContentBlock reports whether a completed request failed with the
// signal-less 429 Antigravity returns for flagged prompt content. Real quota
// and rate limits carry details such as ErrorInfo, RetryInfo or retryDelay.
func isGoogleContentBlock(completion pluginapi.RequestCompletion) bool {
	if completion.Outcome != pluginapi.RequestCompletionFailed || completion.StatusCode != http.StatusTooManyRequests {
		return false
	}
	body := completion.Error
	if !strings.Contains(body, "RESOURCE_EXHAUSTED") {
		return false
	}
	for _, signal := range []string{"details", "retryDelay", "RetryInfo", "ErrorInfo", "QUOTA_EXHAUSTED", "quotaResetTimeStamp"} {
		if strings.Contains(body, signal) {
			return false
		}
	}
	return true
}
