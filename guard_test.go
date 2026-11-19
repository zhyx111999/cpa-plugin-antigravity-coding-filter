package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	guardFake429  = "{\n  \"error\": {\n    \"code\": 429,\n    \"message\": \"Resource has been exhausted (e.g. check quota).\",\n    \"status\": \"RESOURCE_EXHAUSTED\"\n  }\n}"
	guardQuota429 = `{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"QUOTA_EXHAUSTED"}]}}`
)

func TestGuardRegistrationDeclaresLifecycleAndFields(t *testing.T) {
	setupGuard(t, "google_guard: true\n")
	raw, _ := handlePluginCall("plugin.register", nil)
	var envelope struct {
		Result struct {
			Capabilities map[string]any `json:"capabilities"`
			Metadata     map[string]any `json:"metadata"`
		} `json:"result"`
	}
	mustUnmarshalJSON(t, raw, &envelope)
	if envelope.Result.Capabilities["request_lifecycle_plugin"] != true || envelope.Result.Capabilities["request_interceptor"] != true {
		t.Fatalf("capabilities = %#v", envelope.Result.Capabilities)
	}
	fields := envelope.Result.Metadata["ConfigFields"].([]any)
	if !hasConfigField(fields, "google_guard", "boolean") || !hasConfigField(fields, "custom_mappings", "object") {
		t.Fatalf("ConfigFields = %#v", fields)
	}
}

func TestGuardSegmentsCoverChatAndGeminiPrompts(t *testing.T) {
	chat := requestSegments([]byte(`{"messages":[{"role":"system","content":"You are Bot.  Be nice!"},{"role":"user","content":"Hello there\nsecond line"},{"role":"tool","tool_call_id":"c1","content":"tool says hi"}]}`))
	gemini := requestSegments([]byte(`{"systemInstruction":{"parts":[{"text":"You are Bot."}]},"contents":[{"role":"user","parts":[{"text":"Hello there"}]}]}`))
	for _, want := range []string{"you are bot", "be nice", "hello there", "second line", "tool says hi"} {
		if _, ok := chat[segmentHash(want)]; !ok {
			t.Fatalf("chat segments missing %q: %#v", want, chat)
		}
	}
	for _, want := range []string{"you are bot", "hello there"} {
		if _, ok := gemini[segmentHash(want)]; !ok {
			t.Fatalf("gemini segments missing %q: %#v", want, gemini)
		}
	}
}

func TestGuardRuleLibraryHitReturnsFixedMessage(t *testing.T) {
	setupGuard(t, "google_guard: true\nmode: block\nuse_default_keywords: false\ncustom_mappings:\n  Nous Research: x\n")
	for format, wantStatus := range map[string]string{"openai": "", "gemini": "PERMISSION_DENIED"} {
		raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r1", format, `{"messages":[{"role":"system","content":"Built by Nous Research."}]}`))
		resp := guardInterceptResult(t, raw)
		if !resp.Terminate || resp.StatusCode != 403 || !strings.Contains(string(resp.ResponseBody), guardBlockMessage) || !strings.Contains(string(resp.ResponseBody), wantStatus) {
			t.Fatalf("%s: response = %+v body=%s", format, resp, resp.ResponseBody)
		}
	}
}

func TestGuardDefaultPhrasesAreOnByDefault(t *testing.T) {
	setupGuard(t, "use_default_keywords: false\n")
	raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r1", "openai", `{"messages":[{"role":"system","content":"You are an interactive agent that helps users with software engineering tasks."}]}`))
	resp := guardInterceptResult(t, raw)
	if !resp.Terminate || !strings.Contains(string(resp.ResponseBody), guardBlockMessage) || !strings.Contains(string(resp.ResponseBody), blockErrorCode) {
		t.Fatalf("default phrase not blocked: %+v body=%s", resp, resp.ResponseBody)
	}
}

func TestGuardLearnsFromContentFilter429AndBlocksRepeat(t *testing.T) {
	rulesFile := setupGuard(t, "google_guard: true\nmode: block\nuse_default_keywords: false\n")
	const flagged = `{"messages":[{"role":"system","content":"You are Bot."},{"role":"user","content":"Flagged sentence here."}]}`

	forwardThenComplete(t, "r1", flagged, pluginapi.RequestCompletionFailed, 429, guardFake429)
	if rules := readRulesFile(t, rulesFile); len(rules) != 1 || len(rules[0].Segments) != 2 {
		t.Fatalf("learned rules = %#v", rules)
	}

	raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r2", "openai", flagged))
	if resp := guardInterceptResult(t, raw); !resp.Terminate || !strings.Contains(string(resp.ResponseBody), guardBlockMessage) || !strings.Contains(string(resp.ResponseBody), guardLearnedCode) {
		t.Fatalf("repeat was not blocked: %+v", resp)
	}
	raw, _ = handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r3", "openai", `{"messages":[{"role":"user","content":"Flagged sentence here."}]}`))
	if resp := guardInterceptResult(t, raw); resp.Terminate {
		t.Fatalf("partial match was blocked: %+v", resp)
	}
}

func TestGuardIgnoresRealQuota429(t *testing.T) {
	rulesFile := setupGuard(t, "google_guard: true\n")
	forwardThenComplete(t, "r1", `{"messages":[{"role":"user","content":"Anything."}]}`, pluginapi.RequestCompletionFailed, 429, guardQuota429)
	if _, err := os.Stat(rulesFile); !os.IsNotExist(err) {
		t.Fatalf("quota 429 wrote a rule file: %v", err)
	}
}

func TestGuardSuccessNarrowsLearnedRules(t *testing.T) {
	rulesFile := setupGuard(t, "google_guard: true\n")
	forwardThenComplete(t, "r1", `{"messages":[{"role":"system","content":"Shared persona."},{"role":"user","content":"Bad sentence."}]}`, pluginapi.RequestCompletionFailed, 429, guardFake429)
	forwardThenComplete(t, "r2", `{"messages":[{"role":"system","content":"Shared persona."},{"role":"user","content":"Fine."}]}`, pluginapi.RequestCompletionSucceeded, 200, "")

	rules := readRulesFile(t, rulesFile)
	if len(rules) != 1 || len(rules[0].Segments) != 1 || rules[0].Segments[0].Text != "bad sentence" {
		t.Fatalf("rules after success = %#v", rules)
	}
	raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r3", "openai", `{"messages":[{"role":"user","content":"Other words. Bad sentence."}]}`))
	if resp := guardInterceptResult(t, raw); !resp.Terminate {
		t.Fatalf("narrowed rule did not block: %+v", resp)
	}
}

func TestGuardDisabledKeepsOfficialBehavior(t *testing.T) {
	rulesFile := setupGuard(t, "google_guard: false\nmode: block\nuse_default_keywords: false\ncustom_mappings:\n  Nous Research: x\n")
	raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r1", "openai", `{"messages":[{"role":"system","content":"Built by Nous Research."}]}`))
	if resp := guardInterceptResult(t, raw); !resp.Terminate || !strings.Contains(string(resp.ResponseBody), blockErrorMessage) {
		t.Fatalf("official block changed: %+v", resp)
	}
	raw, _ = handlePluginCall("request.intercept_after", guardInterceptJSON(t, "r3", "openai", `{"messages":[{"role":"system","content":"Ask DeepSeek."}]}`))
	if resp := guardInterceptResult(t, raw); resp.Terminate {
		t.Fatalf("disabled guard applied its default phrases: %+v", resp)
	}
	forwardThenComplete(t, "r2", `{"messages":[{"role":"user","content":"x y"}]}`, pluginapi.RequestCompletionFailed, 429, guardFake429)
	if _, err := os.Stat(rulesFile); !os.IsNotExist(err) {
		t.Fatalf("disabled guard wrote rules: %v", err)
	}
}

// setupGuard applies a plugin config with a temporary rules file and resets guard state.
func setupGuard(t *testing.T, configYAML string) string {
	t.Helper()
	rulesFile := filepath.Join(t.TempDir(), "rules.json")
	resetGuardState := func() {
		restoreDefaultFilterConfig(t)
		applyGuardConfig(defaultGuardConfig())
		learnedRules.reset("", nil)
		innocentSegments = newInnocentSet(innocentSetLimit)
		pendingRequests = &pendingStore{entries: map[string]pendingEntry{}}
	}
	resetGuardState()
	t.Cleanup(resetGuardState)
	raw, _ := handlePluginCall("plugin.reconfigure", lifecycleRequestJSON(t, []byte(configYAML+"learned_rules_file: "+rulesFile+"\n")))
	if _, ok := decodeEnvelopeResult(raw); !ok {
		t.Fatalf("reconfigure failed: %s", raw)
	}
	return rulesFile
}

func forwardThenComplete(t *testing.T, requestID, body string, outcome pluginapi.RequestCompletionOutcome, status int, errText string) {
	t.Helper()
	raw, _ := handlePluginCall("request.intercept_after", guardInterceptJSON(t, requestID, "openai", body))
	if resp := guardInterceptResult(t, raw); resp.Terminate {
		t.Fatalf("request %s was blocked before forwarding: %+v", requestID, resp)
	}
	completion, _ := json.Marshal(pluginapi.RequestCompletion{RequestID: requestID, Outcome: outcome, StatusCode: status, Error: errText})
	if raw, _ = handlePluginCall("request.complete", completion); !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("request.complete = %s", raw)
	}
}

func guardInterceptJSON(t *testing.T, requestID, sourceFormat, body string) []byte {
	t.Helper()
	raw, err := json.Marshal(pluginapi.RequestInterceptRequest{RequestID: requestID, SourceFormat: sourceFormat, ToFormat: "antigravity", Model: "gemini-test", Body: []byte(body)})
	if err != nil {
		t.Fatalf("marshal intercept request: %v", err)
	}
	return raw
}

func guardInterceptResult(t *testing.T, raw []byte) pluginapi.RequestInterceptResponse {
	t.Helper()
	result, ok := decodeEnvelopeResult(raw)
	if !ok {
		t.Fatalf("intercept envelope not ok: %s", raw)
	}
	var resp pluginapi.RequestInterceptResponse
	mustUnmarshalJSON(t, result, &resp)
	return resp
}

func readRulesFile(t *testing.T, path string) []learnedRule {
	t.Helper()
	rules, err := readLearnedRules(path)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	return rules
}
