package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

// guardConfig holds the Google guard settings read from the same plugin
// config block as the official filter settings.
type guardConfig struct {
	Enabled   bool
	RulesFile string
}

const defaultLearnedRulesFile = "antigravity-learned-rules.json"

var (
	guardConfigMu      sync.RWMutex
	currentGuardConfig = defaultGuardConfig()
)

func defaultGuardConfig() guardConfig {
	return guardConfig{Enabled: true, RulesFile: defaultLearnedRulesFile}
}

func applyGuardConfig(cfg guardConfig) {
	guardConfigMu.Lock()
	defer guardConfigMu.Unlock()
	currentGuardConfig = cfg
}

func activeGuardConfig() guardConfig {
	guardConfigMu.RLock()
	defer guardConfigMu.RUnlock()
	return currentGuardConfig
}

// parseGuardConfig reads google_guard and learned_rules_file from a lifecycle
// request. Keys the official parser owns are ignored here.
func parseGuardConfig(request []byte) (guardConfig, error) {
	cfg := defaultGuardConfig()
	if len(request) == 0 {
		return cfg, nil
	}
	var req lifecycleRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return guardConfig{}, fmt.Errorf("decode lifecycle request: %w", err)
	}
	if len(strings.TrimSpace(string(req.ConfigYAML))) == 0 {
		return cfg, nil
	}
	var values map[string]any
	if err := yaml.Unmarshal(req.ConfigYAML, &values); err != nil {
		return guardConfig{}, fmt.Errorf("decode config yaml: %w", err)
	}
	if value, exists := values["google_guard"]; exists {
		enabled, ok := value.(bool)
		if !ok {
			return guardConfig{}, fmt.Errorf("google_guard must be a boolean")
		}
		cfg.Enabled = enabled
	}
	if value, exists := values["learned_rules_file"]; exists {
		path, ok := value.(string)
		if !ok || strings.TrimSpace(path) == "" {
			return guardConfig{}, fmt.Errorf("learned_rules_file must be a non-empty string")
		}
		cfg.RulesFile = strings.TrimSpace(path)
	}
	return cfg, nil
}

func guardConfigFields() []pluginapi.ConfigField {
	return []pluginapi.ConfigField{
		{
			Name:        "google_guard",
			Type:        pluginapi.ConfigFieldTypeBoolean,
			Description: "Default true; false restores the official filter behavior. Blocks Antigravity requests that match the rule library (official keywords plus built-in phrases Google flags) or rules learned from Google's content-filter 429 with a fixed message, and learns new rules from those 429s.",
		},
		{
			Name:        "learned_rules_file",
			Type:        pluginapi.ConfigFieldTypeString,
			Description: "Default " + defaultLearnedRulesFile + ". JSON file that stores rules learned from Google's content-filter 429. Relative paths resolve from the CLIProxyAPI working directory.",
		},
	}
}
