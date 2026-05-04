---
status: resolved
file: internal/store/rundb/close_test.go
line: 11
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:9013faecb7ab
review_hash: 9013faecb7ab
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 027: Wrap this scenario in a t.Run("Should ...") subtest.
## Review Comment

The assertions are fine, but this file should still follow the repository's default subtest pattern so additional close-context cases can land without reshaping the test later.

As per coding guidelines, `**/*_test.go`: `Use table-driven tests with subtests (t.Run) as the default pattern`; and `MUST use t.Run("Should...") pattern for ALL test cases`.

## Triage

- Decision: `valid`
- Root cause: `TestRunDBCloseContextDelegatesToSQLiteCloser` also bypasses the repository-standard `t.Run("Should...")` structure for individual test cases.
- Fix approach: wrap the existing assertions in named subtests and include the retry-preservation scenario required by the `RunDB.CloseContext` fix.
- Resolution: `internal/store/rundb/close_test.go` now uses named `Should...` subtests for both delegation and retry-preservation behavior.
- Verification: `go test ./internal/store/rundb` and `make verify`
