---
status: resolved
file: internal/daemon/shutdown.go
line: 165
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:7e9f7f937185
review_hash: 7e9f7f937185
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 021: Consider documenting the null-byte separator convention.
## Review Comment

Using `\x00` as a separator in `joinRunManagerMetricKey`/`splitRunManagerMetricKey` is safe since mode and status values won't contain null bytes, but it's unconventional. A brief comment explaining this choice would improve maintainability.

## Triage

- Decision: `valid`
- Root cause: `joinRunManagerMetricKey` and `splitRunManagerMetricKey` rely on a `\x00` separator, but the invariant behind that choice is implicit in the code.
- Fix approach: add a short comment documenting why the NUL separator is safe for daemon run mode/status metric keys and why it is preferred over printable delimiters.
- Resolution: documented the NUL-separated metric-key convention directly above `joinRunManagerMetricKey`.
- Verification: `go test ./internal/daemon` and `make verify`
