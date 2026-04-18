---
status: resolved
file: internal/cli/daemon.go
line: 103
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134973697,nitpick_hash:1237a8f71f8e
review_hash: 1237a8f71f8e
source_review_id: "4134973697"
source_review_submitted_at: "2026-04-18T19:43:56Z"
---

# Issue 008: Wrap exit-coded errors with operation context before returning.
## Review Comment

Lines 103, 117, 144, and 152 pass raw errors directly to `withExitCode(2, err)`. Wrapping each site with contextual `fmt.Errorf` makes failures diagnosable without changing behavior.

---

## Triage

- Decision: `valid`
- Root cause: multiple `daemon status` and `daemon stop` paths pass raw probe/client-construction errors directly into `withExitCode(2, err)`, which drops the operation context needed to diagnose whether readiness probing or client creation failed.
- Fix plan: wrap each affected error with operation-specific context before applying the exit code wrapper.
- Resolution: `internal/cli/daemon.go` now wraps the affected probe/client-construction failures with operation context, and `internal/cli/daemon_commands_test.go` covers the wrapped error messages plus stable exit code `2`.
