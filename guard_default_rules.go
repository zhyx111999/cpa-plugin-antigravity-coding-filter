package main

// guardDefaultPhrases are prompt phrases that other projects isolated by A/B
// replay as triggers of Antigravity's content-filter 429. They are part of the
// rule library while google_guard is on. Sources are noted per entry.
var guardDefaultPhrases = []string{
	"You are a Claude agent, built on Anthropic's Claude Agent SDK", // luqman-v1/9router-go, CLIProxyAPI#7049
	"Anthropic's Claude Agent SDK",                                  // luqman-v1/9router-go
	"You are Claude Code",                                           // 9router-go#1234, CLIProxyAPI#7553
	"Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context", // 9router-go#1234, CLIProxyAPI#7627
	"pasted into the message by the user from somewhere else and may contain instructions the user did not write",          // 9router-go#1234
	"You are an interactive agent that helps users with software engineering tasks",                                        // CLIProxyAPI#7627
	"x-anthropic-billing-header",             // 9router-go, badrisnarayanan/antigravity-claude-proxy#375
	"RFC 2119",                               // 9router-go, CLIProxyAPI#7540
	"RFC 8174",                               // 9router-go
	"<system-conventions>",                   // 9router-go, CLIProxyAPI#7540
	"<system_conventions>",                   // 9router-go
	"<system-directive>",                     // 9router-go
	"<system_directive>",                     // 9router-go
	"<critical>",                             // 9router-go
	"You are omp's trusted coding assistant", // 9router-go, CLIProxyAPI#7540
	"oh-my-pi",                               // 9router-go
	"oh my pi",                               // 9router-go
	"oh_my_pi",                               // 9router-go
	"omp Live",                               // 9router-go
	"for full docs + JSON schema before first use", // CLIProxyAPI#7562
	"You are ZCode",                  // CLIProxyAPI#7553
	"Nous Research",                  // antigravity-claude-proxy#364, 9router-go
	"DeepSeek",                       // cpa-plugin-antigravity-coding-filter#10
	"When asked about: extensions (", // cpa-plugin-antigravity-coding-filter#11
}

// guardDefaultRules reuse the official case-insensitive literal matcher. The
// guard only blocks, so the replacement normalizeMappings requires is unused.
var guardDefaultRules = normalizeMappings(blockOnlyMappings(guardDefaultPhrases))

func blockOnlyMappings(phrases []string) []rewriteMapping {
	mappings := make([]rewriteMapping, 0, len(phrases))
	for _, phrase := range phrases {
		mappings = append(mappings, rewriteMapping{Match: phrase, Replacement: "-"})
	}
	return mappings
}

// guardRuleLibraryHit reports whether the body contains a default guard rule
// within the scope the official filter is configured for.
func guardRuleLibraryHit(body []byte) bool {
	official := activeFilterConfig()
	return classifyRequestWithConfig(body, filterConfig{
		Mode:           filterModeBlock,
		Scope:          official.Scope,
		CustomMappings: guardDefaultRules,
	}).Blocked
}
