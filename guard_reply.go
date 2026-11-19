package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	guardBlockMessage = "您的请求中有谷歌标记的特征，请修改提示词后重试"
	// guardLearnedCode marks hits on rules learned from Google's 429; rule-library
	// hits keep the official blockErrorCode.
	guardLearnedCode = "google_flagged_prompt"
	guardBlockStatus = http.StatusForbidden
)

// guardBlockedResponse terminates the request locally. 403 is not retried by
// common SDKs, and nothing reaches Google, so no credential is touched.
func guardBlockedResponse(sourceFormat, code string) pluginapi.RequestInterceptResponse {
	return pluginapi.RequestInterceptResponse{
		Terminate:       true,
		StatusCode:      guardBlockStatus,
		ResponseHeaders: jsonHeaders(),
		ResponseBody:    guardBlockedBody(sourceFormat, code),
	}
}

// guardBlockedBody shapes the error like the client's own protocol so its SDK
// surfaces the message.
func guardBlockedBody(sourceFormat, code string) []byte {
	format := strings.ToLower(sourceFormat)
	var payload any
	switch {
	case strings.Contains(format, "gemini"):
		payload = map[string]any{"error": map[string]any{"code": guardBlockStatus, "message": guardBlockMessage, "status": "PERMISSION_DENIED", "reason": code}}
	case strings.Contains(format, "claude") || strings.Contains(format, "anthropic"):
		payload = map[string]any{"type": "error", "error": map[string]any{"type": "permission_error", "message": guardBlockMessage, "code": code}}
	default:
		payload = map[string]any{"error": map[string]any{"message": guardBlockMessage, "type": "invalid_request_error", "code": code}}
	}
	raw, _ := json.Marshal(payload)
	return raw
}
