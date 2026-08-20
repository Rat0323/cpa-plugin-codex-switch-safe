# Codex Switch Safe

[![Build](https://github.com/Rat0323/cpa-plugin-codex-switch-safe/actions/workflows/build.yml/badge.svg)](https://github.com/Rat0323/cpa-plugin-codex-switch-safe/actions/workflows/build.yml)
[![Latest release](https://img.shields.io/github/v/release/Rat0323/cpa-plugin-codex-switch-safe)](https://github.com/Rat0323/cpa-plugin-codex-switch-safe/releases/latest)
[![License](https://img.shields.io/github/license/Rat0323/cpa-plugin-codex-switch-safe)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/Rat0323/cpa-plugin-codex-switch-safe)](go.mod)

A native CLIProxyAPI (CPA) request interceptor that prevents Codex encrypted
reasoning state from crossing credential or model routes during dynamic
upstream switching.

Codex Switch Safe passes encrypted reasoning through unchanged on the same
selected route. When the route changes, is unknown, expires, or becomes
ambiguous, it removes only unsafe top-level route-bound state. It never decrypts
encrypted content and does not alter user prompts, tools, nested agent messages,
or model selection.

## What it does

- Preserves valid encrypted reasoning on the same CPA credential/model route.
- Strips unsafe top-level `reasoning` items and `previous_response_id` before a
  request reaches a different or unknown route.
- Blocks unsafe `compaction` by default, or strips it when explicitly configured
  for availability.
- Commits route changes only after CPA reports a successful request lifecycle
  outcome; failed retries and failovers do not advance the route.
- Keeps bounded, process-local state and emits privacy-safe diagnostics through
  CPA's existing logging system.
- Ignores non-Codex targets and leaves unrelated request content untouched.

The plugin cannot decrypt ciphertext from another credential. Its purpose is to
prevent foreign encrypted state from reaching that credential in the first
place.

## Quick start

Requirements:

- CLIProxyAPI `7.2.130` or later.
- A release asset matching the CPA host operating system and architecture.
- A CPA configuration with plugins enabled.

Install Codex Switch Safe from CPA's plugin store when the listing is available,
or download the appropriate archive from the
[latest release](https://github.com/Rat0323/cpa-plugin-codex-switch-safe/releases/latest).
Each archive contains one root-level platform library:

| Platform | Architectures | Library |
| --- | --- | --- |
| Windows | `amd64`, `arm64` | `codex-switch-safe.dll` |
| Linux | `amd64`, `arm64` | `codex-switch-safe.so` |
| macOS | `amd64`, `arm64` | `codex-switch-safe.dylib` |

For a manual installation, extract the library into CPA's configured `plugins`
directory and restart CPA. Release archives use this naming convention:

```text
codex-switch-safe_<version>_<goos>_<goarch>.zip
```

The release also includes `checksums.txt` for archive verification and
`artifact-manifest.json` for the source commit, packaged-library size, and
packaged-library SHA-256.

## Behavior

| Request and route state | Default behavior |
| --- | --- |
| Non-Codex target | Pass through without intervention |
| No top-level route-bound state | Pass through unchanged |
| Encrypted state on the same committed route | Pass through unchanged |
| Changed or unknown route | Strip reasoning and prior response ID |
| Unsafe compaction (`block`) | Return HTTP 409 |
| Unsafe compaction (`strip`) | Strip and continue |
| Failed lifecycle attempt | Do not commit the route |
| Successful lifecycle attempt | Commit the route |

Only top-level Responses input items are inspected for removal. Nested
`agent_message` content and tool payloads are not rewritten.

## Configuration

```yaml
plugins:
  enabled: true
  configs:
    codex-switch-safe:
      enabled: true
      compaction_policy: block
      state_ttl: 4h
      max_sessions: 4096
      max_pending: 8192
      diagnostics: actions
```

| Field | Default | Accepted values | Purpose |
| --- | --- | --- | --- |
| `enabled` | `true` | `true`, `false` | Plugin instance toggle |
| `compaction_policy` | `block` | `block`, `strip` | Compaction handling |
| `state_ttl` | `4h` | `1m` through `24h` | Route binding lifetime |
| `max_sessions` | `4096` | `1` through `65536` | Session entry cap |
| `max_pending` | `8192` | `1` through `65536` | Pending attempt cap |
| `diagnostics` | `actions` | `off`, `actions`, `debug` | Plugin logging level |

`block` is the recommended compaction policy. It avoids silently discarding
compaction context. Use `strip` only when continuing without that context is
preferable to returning a conflict.

The policies differ only when unsafe top-level `compaction` is present. They
handle ordinary encrypted reasoning and same-route continuations identically:

| Situation | `block` | `strip` |
| --- | --- | --- |
| Same committed credential/model route | Preserve route-bound state and continue | Preserve route-bound state and continue |
| Changed/unknown route with reasoning but no compaction | Strip unsafe reasoning and prior response ID, then continue | Strip unsafe reasoning and prior response ID, then continue |
| Changed/unknown route with compaction | Return HTTP 409 without sending the request upstream | Strip reasoning, compaction, and prior response ID, then continue |
| Main tradeoff | Maximum continuity safety; the turn may require retrying on the original route or starting clean | Higher availability; compressed context may be discarded on a route switch |

`block` is appropriate when preserving compressed conversation context matters
more than completing the current turn. `strip` is appropriate when automatic
credential failover should keep working even if the new route must continue
without the old route's compressed state.

CPA `7.2.130` exposes plugin-owned configuration fields without a separate
default-value property. Management screens may therefore display these fields as
blank until an override is saved. Blank or omitted fields are valid: the plugin
applies the runtime defaults shown above.

The CPA plugin priority controls interceptor order. If another request
interceptor rewrites Codex request bodies, give Codex Switch Safe a lower
priority so it runs afterward and performs the final safety check.

## Diagnostics and privacy

| Level | Records |
| --- | --- |
| `off` | No plugin diagnostics |
| `actions` | Protection actions and their final lifecycle outcomes |
| `debug` | Everything in `actions`, plus safe pass-through decisions |

`actions` is the default and is suitable for normal operation. Use `debug` for
short-term troubleshooting when you need to confirm whether the plugin observed
and passed or sanitized a request.

Diagnostics include the CPA request ID, action, outcome, removal counts, and
short process-local HMAC references for route/session correlation. They never
include:

- API keys or authorization headers.
- Raw selected-auth, session, thread, or conversation IDs.
- Request bodies, encrypted content, reasoning text, or user conversation text.

The plugin has no network access and does not write files. It stores routing
state only in memory and uses a random process-local HMAC key for opaque item
fingerprints and diagnostic references. References cannot be correlated across
CPA restarts.

## Safety model

- **Stable session identity:** CPA `execution_session_id` first, then Codex
  session/thread headers, payload metadata, and `prompt_cache_key` fallbacks.
  Per-turn IDs are ignored.
- **Route identity:** CPA `selected_auth_id`, selected model, and
  `ToFormat: codex`.
- **Retry-aware lifecycle:** a request ID keeps only its latest after-auth
  candidate, and a route is committed only after outcome `succeeded`.
- **Failover safety:** failed, rejected, canceled, and expired attempts do not
  advance the committed route.
- **Concurrency isolation:** independent execution sessions remain isolated.
  Same-session cross-route concurrency taints the session until a clean
  successful turn establishes a new route.
- **Bounded memory:** configurable TTL and entry caps constrain process-local
  state. An overflowed retired-item barrier causes conservative full stripping
  for that session until its state expires.

The plugin does not serialize model requests. It deliberately prefers a clean
continuation over guessing which concurrent result is valid.

## Limitations

The plugin cannot distinguish a server-side key rotation that reuses the same
CPA auth ID until the upstream reports an invalid signature. On that signal it
invalidates the matching in-memory route and forces the next continuation to be
clean.

Removing route-bound state can reduce available reasoning context for a turn,
but it does not change the user prompt, tools, subagent messages, or model
selection. Process-local routing state is reset whenever CPA restarts.

## Development

```powershell
$go = 'C:\Program Files\Go\bin\go.exe'
& $go test ./...
& $go test -race ./...
& $go vet ./...
```

Native builds require a C compiler because CPA loads the Go shared library
through the native ABI. CI tests the plugin and publishes Linux, macOS, and
Windows release assets from version tags.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development checks, pull request
expectations, and the release process. User-facing changes are tracked in
[CHANGELOG.md](CHANGELOG.md).

## Security

Report security-sensitive findings through GitHub private vulnerability
reporting instead of a public issue. See [SECURITY.md](SECURITY.md) for supported
versions and disclosure guidance.

## License

MIT. See [LICENSE](LICENSE).
