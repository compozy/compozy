---
status: resolved
file: internal/cli/reviews_exec_daemon_additional_test.go
line: 1389
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:0dac5a1eee8f
review_hash: 0dac5a1eee8f
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 015: Align this new subtest with enforced test conventions.
## Review Comment

Please switch to the `Should...` naming convention and mark it parallel if independence is intended.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases` and `Use t.Parallel() for independent subtests`.

## Triage

- Decision: `valid`
- Root cause: the newly added subtest does not follow the file's `Should...` naming convention and it does not share state with sibling cases, so it can safely opt into subtest parallelism.
- Fix approach: rename the subtest to the enforced `Should...` form and add `t.Parallel()` inside the case.
- Resolution: renamed the subtest to `Should wait for a durable terminal snapshot after a terminal watch event` and marked it parallel.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the subtest update.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
