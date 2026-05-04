---
status: resolved
file: internal/cli/daemon_commands_test.go
line: 1132
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:da713f3e0cc2
review_hash: da713f3e0cc2
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 013: Missing t.Parallel() for independent test.
## Review Comment

This test uses `t.TempDir()` for filesystem isolation and doesn't share state with other tests, so it should run in parallel.

## Triage

- Decision: `valid`
- Root cause: `TestLaunchCLIDaemonProcessFailsWhenDaemonLogFileCannotBeOpened` uses only fresh temp paths and does not mutate the CLI global override hooks, so it is an independent case that can run concurrently with the rest of the file.
- Fix approach: add `t.Parallel()` at the start of the test.
- Resolution: added `t.Parallel()` to the independent daemon-log-open failure test.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the change.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
