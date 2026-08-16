# Contributing

Thanks for helping improve Codex Switch Safe.

## Before opening a change

- Use an issue for reproducible bugs or behavior proposals.
- Use private vulnerability reporting for security-sensitive findings.
- Keep changes focused on Codex route-bound state safety and CPA plugin
  integration.
- Never include credentials, raw request bodies, encrypted reasoning, or user
  conversation content in tests, issues, commits, or logs.

## Development checks

```powershell
$go = 'C:\Program Files\Go\bin\go.exe'
& $go test ./...
& $go test -race ./...
& $go vet ./...
```

Native shared-library builds require a C compiler. Pull requests run tests and
all six supported platform builds in GitHub Actions.

## Pull requests

- Explain the behavior change and its safety implications.
- Add focused regression tests for state, retry, failover, or sanitization
  changes.
- Update README configuration or diagnostics documentation when behavior changes.
- Add user-facing changes under `[Unreleased]` in `CHANGELOG.md`.
- Keep version bumps in a dedicated release-preparation pull request.

## Releases

A release-preparation pull request moves relevant changelog entries from
`[Unreleased]` into a dated version section and synchronizes the version in
`main.go`, `Makefile`, and `marketplace/registry-entry.json`.

After that pull request is merged and verified, create and push the matching
`v<version>` tag. The release workflow validates metadata and checksums, creates
a draft release, uploads all platform assets, verifies the asset list, and then
publishes it.
