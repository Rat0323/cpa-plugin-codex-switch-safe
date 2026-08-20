# Changelog

All notable user-facing changes to Codex Switch Safe are documented here.
The project follows semantic versioning, and release automation publishes the
matching version section verbatim.

## [Unreleased]

## [0.2.1] - 2026-08-21

### Maintenance

- Made pure-Go unit tests compile without a host C ABI, so CI explicitly tests
  both `CGO_ENABLED=0` and `CGO_ENABLED=1` configurations.
- Added release artifact manifests containing the source commit plus archive
  and packaged-library SHA-256 hashes for installed-binary verification.

## [0.2.0] - 2026-08-15

### Highlights

- Added privacy-safe diagnostics for route decisions, protection actions, and
  request lifecycle outcomes.
- Added route-scoped retired-item barriers so a rejected failover candidate
  does not retire valid state from the original route.
- Made route tracking retry-aware and lifecycle-aware. Failed, rejected,
  canceled, and expired attempts do not advance the committed route.
- Preserved encrypted reasoning on the same selected CPA credential/model route
  while stripping only unsafe top-level route-bound state during switching.
- Expanded regression coverage for encrypted reasoning, compaction, retries,
  failover, concurrency, and diagnostics.

### Behavior

- Same-route encrypted reasoning passes through unchanged.
- Unknown, changed, expired, or ambiguous routes are handled conservatively so
  foreign encrypted state is not sent to a new upstream.
- Unsafe compaction returns HTTP 409 by default. Setting
  `compaction_policy: strip` continues without unsafe compaction context.
- A successful lifecycle outcome is required before a route and its
  route-bound item fingerprints are committed.
- Diagnostics default to `actions`; `debug` adds safe pass-through decisions,
  and `off` disables plugin diagnostics.

### Upgrade notes

- Requires CLIProxyAPI `7.2.130` or newer.
- Existing plugin configuration remains compatible.
- Release assets cover Linux, macOS, and Windows on `amd64` and `arm64`.

## [0.1.1] - 2026-08-14

### Fixed

- Committed selected credential routes only after successful upstream requests.
- Prevented failed failover candidates from retiring valid reasoning items.
- Added process-local keyed fingerprints for top-level route-bound items.
- Improved stable session identity detection and bounded-state failure handling.
- Completed live dual-provider switching validation and six-platform builds.

### Compatibility

- Requires CLIProxyAPI `7.2.130` or newer.
- Users of `v0.1.0` should upgrade to this release.

## [0.1.0] - 2026-08-14

Initial release. This version is deprecated because retry and session edge cases
were corrected in `v0.1.1`.

[Unreleased]: https://github.com/Rat0323/cpa-plugin-codex-switch-safe/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/Rat0323/cpa-plugin-codex-switch-safe/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Rat0323/cpa-plugin-codex-switch-safe/releases/tag/v0.2.0
[0.1.1]: https://github.com/Rat0323/cpa-plugin-codex-switch-safe/releases/tag/v0.1.1
[0.1.0]: https://github.com/Rat0323/cpa-plugin-codex-switch-safe/releases/tag/v0.1.0
