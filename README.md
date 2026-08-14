# codex-switch-safe

`codex-switch-safe` is a native CPA request interceptor for Codex Responses
traffic. It preserves encrypted reasoning state only when CPA selected the same
credential/model route for the same conversation. When the route is unknown,
changes, expires, or becomes ambiguous because of cross-route concurrency, the
plugin removes only top-level `reasoning` and `compaction` input items plus
`previous_response_id`. Nested `agent_message` content and tool payloads are
left untouched.

The plugin does not decrypt, inspect, persist, or forward encrypted reasoning
content. It keeps state in memory only and fails closed when the request cannot
be safely associated with a stable session and selected auth identity.

## Why this exists

Codex encrypted reasoning is bound to the upstream credential/session that
created it. An unprefixed dynamic model pool can send the next turn to another
credential, which causes `invalid_encrypted_content` or
`thinking_signature_invalid`. Removing route-bound state avoids sending a
foreign ciphertext to the new upstream, at the cost of asking the model to
re-establish reasoning for that turn.

## Safety model

- Stable session identity: CPA `execution_session_id` first, then Codex
  session/thread headers, payload metadata, and `prompt_cache_key` fallbacks.
  Per-turn IDs are ignored.
- Route identity: CPA `selected_auth_id`, selected model, and `ToFormat: codex`.
- Retry aware: a request ID keeps only its latest after-auth candidate; a route
  is committed only after lifecycle outcome `succeeded`. Route-bound item
  fingerprints are also committed only for that successful candidate, so a
  failed failover does not retire valid state from the original route.
- Failover aware: failed, rejected, canceled, and expired attempts do not
  advance the committed route.
- Subagent aware: independent execution sessions are isolated. Same-session
  cross-route concurrency taints the session until a clean successful turn
  establishes a new route.
- Compaction: default `block` returns HTTP 409 on an unsafe route change. Set
  `compaction_policy: strip` only when availability is more important than
  preserving compaction context.
- Bounded memory: state is process-local with configurable TTL and entry caps.
  Each session keeps a bounded retired-item barrier; if it overflows, that
  session conservatively full-strips top-level reasoning/compaction until its
  state expires instead of allowing an old item to be forgotten and replayed.

This plugin does not serialize model requests and cannot make ciphertext from a
different credential decryptable. It deliberately prefers a clean continuation
over guessing which concurrent result is valid.

## Installation

Use CPA's plugin store or place the platform library in the configured `plugins`
directory. The official store release assets are named:

`codex-switch-safe_<version>_<goos>_<goarch>.zip`

Each archive contains exactly one root-level dynamic library and the release
contains `checksums.txt`.

Minimum CPA version: `7.2.130` (request lifecycle completion support).

## Configuration

```yaml
plugins:
  enabled: true
  configs:
    codex-switch-safe:
      enabled: true
      state_ttl: 4h
      max_sessions: 4096
      max_pending: 8192
      compaction_policy: block
```

`state_ttl` accepts `1m` through `24h`. `compaction_policy` is `block` or
`strip`; `block` is the recommended default.

The plugin has no network access, does not write files, and never stores raw
request bodies, tokens, authorization headers, or encrypted content. It uses a
random process-local HMAC key to compare opaque top-level item fingerprints.

## Development

```powershell
$go = 'C:\Program Files\Go\bin\go.exe'
& $go test ./...
& $go vet ./...
```

Native builds require a C compiler because CPA loads the Go shared library via
the native ABI. CI produces Linux, macOS, and Windows release assets.

## Limitations

The plugin cannot distinguish a server-side key rotation that reuses the same
CPA auth ID until the upstream reports an invalid signature. On that signal it
invalidates the matching in-memory route and forces the next continuation to be
clean. Removing route-bound state can reduce available reasoning context for a
turn, but it does not change the user prompt, tools, subagent messages, or model
selection.

## License

MIT. See [LICENSE](LICENSE).
