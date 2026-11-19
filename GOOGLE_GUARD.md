# Google guard

An add-on to the official filter for Antigravity requests. It is on by default; `google_guard: false` restores the official behavior exactly.

## Why

Antigravity answers certain prompt content with a signal-less `429 RESOURCE_EXHAUSTED` (no `details`, `ErrorInfo`, `RetryInfo` or `Retry-After`). It is not quota: the same credential returns 200 seconds later for neutral content. Proxies that treat it as quota cool the credential down and replay the same request on other credentials, so one flagged prompt can take a whole pool out. See router-for-me/CLIProxyAPI#7553, #7560, #7562, #7614, #7627 and luqman-v1/9router-go#1233.

## Behavior

For requests whose selected upstream is Antigravity:

1. **Rule library.** A request that matches the official keywords (block mode) or the built-in phrases in `guard_default_rules.go` is answered locally with HTTP 403 and `您的请求中有谷歌标记的特征，请修改提示词后重试`. Nothing is sent to Google, so no credential is touched. The error code stays `blocked_by_antigravity_coding_filter`.
2. **Learning.** Every forwarded request's prompt text is split into normalized sentences and lines. When the host reports Google's signal-less 429 for it, the sentences that never appeared in a successful request become a learned rule in `learned_rules_file`. Later requests that contain all sentences of a learned rule get the same 403 message (code `google_flagged_prompt`). Sentences later seen in a successful request are pruned from rules, so rules narrow toward the flagged sentence.

The first request that Google rejects still returns Google's 429; the plugin can only observe that response. Pair the plugin with a host rule that stops the request without rotation or cooldown:

```yaml
oauth:
  request-scoped-errors:
    antigravity:
      - status: 429
        match-regexr:
          - '"status":\s*"RESOURCE_EXHAUSTED"\s*\}\s*\}\s*$'
        action: "stop"
```

## Settings

| Key | Default | Meaning |
|---|---|---|
| `google_guard` | `true` | `false` turns the guard off and keeps the official filter only. |
| `learned_rules_file` | `antigravity-learned-rules.json` | Learned rules, relative to the CLIProxyAPI working directory. Edit or delete entries and reconfigure the plugin to apply. |

The official `mode`, `scope`, `use_default_keywords` and `custom_mappings` keep their meaning; the built-in phrases use the same `scope`.
