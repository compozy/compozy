---
status: resolved
file: internal/api/httpapi/browser_middleware.go
line: 297
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:ab81b93529e5
review_hash: ab81b93529e5
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 008: Use type assertion instead of error string matching for net.SplitHostPort.
## Review Comment

Branching on `strings.Contains(err.Error(), "missing port in address")` is brittle—error messages are not documented as stable API and can change across Go versions. Instead, use a type assertion to `*net.AddrError` to check the error condition reliably:

## Triage

- Decision: `valid`
- Notes:
  - `splitAuthority` currently branches on `err.Error()` text from `net.SplitHostPort`, which is brittle across Go versions.
  - Root cause: the implementation inspects the formatted error string instead of the typed `*net.AddrError`.
  - Intended fix: use `errors.As` with `*net.AddrError` and match the `"missing port in address"` condition via the typed error fields.

## Resolution

- Replaced the string-based `SplitHostPort` error check with `errors.As` against `*net.AddrError` and matched the typed `"missing port in address"` condition.
- Verified with `make verify`.
