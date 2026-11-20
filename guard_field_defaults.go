package main

import "encoding/json"

// configFieldDefaults are the values the plugin uses when a key is omitted.
// They are published as each config field's "Default" so management clients
// can show the effective state of omitted keys instead of a blank or off
// control. Hosts whose ConfigField has no Default ignore the extra key.
var configFieldDefaults = map[string]any{
	"mode":                 string(filterModeBlock),
	"scope":                []string{string(scopeSystem)},
	"use_default_keywords": true,
	"custom_mappings":      map[string]string{},
	"google_guard":         true,
	"learned_rules_file":   defaultLearnedRulesFile,
}

// withConfigFieldDefaults adds "Default" to every registered config field that
// has a known default.
func withConfigFieldDefaults(fields []any) []any {
	for index, field := range fields {
		object, ok := toConfigFieldObject(field)
		if !ok {
			continue
		}
		name, _ := object["Name"].(string)
		if value, known := configFieldDefaults[name]; known {
			object["Default"] = value
		}
		fields[index] = object
	}
	return fields
}

// toConfigFieldObject returns a field decoded from the official registration
// or one of the guard's own pluginapi.ConfigField values as a JSON object.
func toConfigFieldObject(field any) (map[string]any, bool) {
	if object, ok := field.(map[string]any); ok {
		return object, true
	}
	raw, err := json.Marshal(field)
	if err != nil {
		return nil, false
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, false
	}
	return object, true
}
