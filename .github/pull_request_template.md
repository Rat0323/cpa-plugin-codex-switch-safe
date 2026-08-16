# Pull Request

## Summary

Describe what changes and why.

## Safety impact

Describe any effect on route identity, encrypted reasoning, compaction, retries,
failover, diagnostics, or request sanitization. Write `None` when not applicable.

## Validation

- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] User-facing changes are documented in `CHANGELOG.md` or are not applicable
- [ ] No credentials, request bodies, encrypted content, or conversation text
      are included
