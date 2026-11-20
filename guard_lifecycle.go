package main

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
)

const guardVersionSuffix = "+google-guard"

// guardLifecycle validates the guard settings, runs the official lifecycle,
// then applies the guard settings. Either failure keeps the previous config.
func guardLifecycle(request []byte) []byte {
	cfg, err := parseGuardConfig(request)
	if err != nil {
		return mustErrorEnvelope("invalid_config", err.Error())
	}
	var rules []learnedRule
	if cfg.Enabled {
		if rules, err = readLearnedRules(cfg.RulesFile); err != nil {
			return mustErrorEnvelope("invalid_config", err.Error())
		}
	}

	raw := handlePluginLifecycle(request)
	if _, ok := decodeEnvelopeResult(raw); !ok {
		return raw
	}
	applyGuardConfig(cfg)
	learnedRules.reset(cfg.RulesFile, rules)
	return withGuardRegistration(raw)
}

// withGuardRegistration adds the lifecycle capability and guard config fields
// to the official registration response.
func withGuardRegistration(raw []byte) []byte {
	result, ok := decodeEnvelopeResult(raw)
	if !ok {
		return raw
	}
	var registration map[string]any
	if err := json.Unmarshal(result, &registration); err != nil {
		return raw
	}
	if capabilities, ok := registration["capabilities"].(map[string]any); ok {
		capabilities["request_lifecycle_plugin"] = true
	}
	if metadata, ok := registration["metadata"].(map[string]any); ok {
		metadata["Version"] = pluginVersion + guardVersionSuffix
		fields, _ := metadata["ConfigFields"].([]any)
		for _, field := range guardConfigFields() {
			fields = append(fields, field)
		}
		metadata["ConfigFields"] = withConfigFieldDefaults(fields)
	}
	return mustEnvelope(registration)
}

// decodeEnvelopeResult returns the result of a successful plugin envelope.
func decodeEnvelopeResult(raw []byte) (json.RawMessage, bool) {
	var envelope pluginabi.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil || !envelope.OK {
		return nil, false
	}
	return envelope.Result, true
}
