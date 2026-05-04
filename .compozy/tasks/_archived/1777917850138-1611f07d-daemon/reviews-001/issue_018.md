---
status: resolved
file: internal/cli/commands.go
line: 13
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:59c2c6f7e267
review_hash: 59c2c6f7e267
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 018: Remove the ignored dispatcher parameter from these private builders.
## Review Comment

Each helper now hardwires the daemon runner and never uses `*kernel.Dispatcher`. Keeping it in the signature preserves dead API surface and makes the migration look partial. Drop the parameter from the `WithDefaults` helpers and from the wrappers that only thread it through.

Also applies to: 43-43, 143-143

## Triage

- Decision: `VALID`
- Root cause: several private command-builder helpers still accept and thread through an unused dispatcher parameter, leaving dead API surface and misleading signatures after the daemon migration.
- Fix plan: remove the unused parameter from the affected helpers and update the thin wrappers/callers/tests that only forwarded `nil`. This requires a minimal follow-on edit in `internal/cli/root.go` and a small set of CLI tests outside the listed code-file set so the build remains consistent.
- Resolution: Implemented and verified with `make verify`.
