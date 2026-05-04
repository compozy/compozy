---
status: resolved
file: internal/store/sqlite_test.go
line: 11
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:d9e9d9ead622
review_hash: d9e9d9ead622
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 030: Restructure these into table-driven subtests.
## Review Comment

This file is exercising one small outcome matrix for `CloseSQLiteDatabase`, so a single `TestCloseSQLiteDatabase` with `t.Run("Should ...")` cases would fit the repo test pattern better and make the remaining branches easier to cover.

As per coding guidelines, `**/*_test.go`: `Table-driven tests with subtests (t.Run) as the default pattern`; and `MUST use t.Run("Should...") pattern for ALL test cases`.

## Triage

- Decision: `valid`
- Root cause: `internal/store/sqlite_test.go` currently encodes a small behavior matrix as separate top-level tests instead of repository-standard `t.Run("Should...")` cases.
- Fix approach: convert the file to a table-driven/subtest structure while preserving the current checkpoint/close assertions.
- Resolution: `internal/store/sqlite_test.go` now groups the close-path matrix under named `Should...` subtests without changing the assertions.
- Verification: `go test ./internal/store` and `make verify`
