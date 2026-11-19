package main

// Google guard: an add-on to the official filter, on by default
// (google_guard). For Antigravity requests it
//   - answers rule-library hits (official keywords and the built-in phrases
//     Google flags) and learned-rule hits with one fixed message instead of
//     forwarding them, and
//   - learns a rule whenever Google rejects a forwarded request with its
//     signal-less content-filter 429.
// With google_guard: false every call takes the official path unchanged.

import "github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"

// handleGuardCall is the single hook called from handlePluginCall. It reports
// false for methods the official handler should process.
func handleGuardCall(method string, request []byte) ([]byte, bool) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return guardLifecycle(request), true
	case pluginabi.MethodRequestInterceptAfter:
		if !activeGuardConfig().Enabled {
			return nil, false
		}
		return guardInterceptAfter(request), true
	case pluginabi.MethodRequestComplete:
		return guardComplete(request), true
	default:
		return nil, false
	}
}
