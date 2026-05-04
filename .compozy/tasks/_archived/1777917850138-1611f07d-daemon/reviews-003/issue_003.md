---
status: resolved
file: internal/cli/migrate_command_test.go
line: 37
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4135415269,nitpick_hash:778e26b9ae1d
review_hash: 778e26b9ae1d
source_review_id: "4135415269"
source_review_submitted_at: "2026-04-19T03:14:21Z"
---

# Issue 003: Align failure messages with the new command name.
## Review Comment

The command invocations are updated to `tasks validate`, but assertion messages still say `validate-tasks`. Updating the text will make failures less confusing.

Also applies to: 55-66

## Triage

- Decision: `valid`
- Root cause: `internal/cli/migrate_command_test.go` already invokes `tasks validate`, but four failure messages still reference the retired `validate-tasks` command name.
- Fix approach: update the stale assertion text so test failures describe the current command accurately; no production-code change is needed.
- Resolution: updated all four stale assertion messages in `internal/cli/migrate_command_test.go` to use `tasks validate`, matching the command under test.
- Regression coverage: the existing `TestMigrateCommandPrintsUnmappedTypeSummaryAndValidateFailsUntilFixed` and `TestValidateTasksCommandPassesCommittedACPFixtures` cases still exercise the same CLI paths; this change only corrected their failure text.
- Verification: `go test ./internal/cli -run 'Test(MigrateCommandPrintsUnmappedTypeSummaryAndValidateFailsUntilFixed|ValidateTasksCommandPassesCommittedACPFixtures)$' -count=1` passed. A fresh `make verify` then failed during lint outside this batch at `internal/cli/run_observe.go:141`, where `goconst` flagged another repeated `"canceled"` literal unrelated to the scoped `internal/cli/migrate_command_test.go` message-only change.
- Verification note: `git diff -- internal/cli/run_observe.go` shows concurrent, unrelated edits adding `isTerminalObservedRunStatus` and `isTerminalObservedJobStatus`; this batch left that out-of-scope lint failure untouched.
