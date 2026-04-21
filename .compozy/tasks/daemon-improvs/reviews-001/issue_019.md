---
status: resolved
file: internal/daemon/run_integrity.go
line: 31
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:5969d0259a05
review_hash: 5969d0259a05
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 019: Wrap these integrity-store failures with run context.
## Review Comment

The bare returns here lose the run ID and phase (`acquire`, `get`, `upsert`), which will make daemon-side integrity failures much harder to trace once they bubble into logs.

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

Also applies to: 70-89

## Triage

- Decision: `valid`
- Root cause: `persistRuntimeIntegrity` and `loadRunIntegrity` return bare acquire/get/upsert errors, which drops the run id and phase context that daemon logs need when integrity persistence fails.
- Fix approach: wrap those failures with the run id and operation context (`acquire`, `get`, `upsert`) before returning.
- Resolution: wrapped runtime-integrity acquire/get/upsert failures with run-id-specific context and added a daemon regression test that exercises the wrapped `loadRunIntegrity` get-error path.
- Regression coverage: `go test ./internal/daemon` passed after the new error-context test landed.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
