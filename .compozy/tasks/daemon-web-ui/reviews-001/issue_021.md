---
status: resolved
file: internal/store/globaldb/read_queries_test.go
line: 14
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:c91fb50f862f
review_hash: c91fb50f862f
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 021: Consider logging or checking db.Close() error in test cleanup.
## Review Comment

Per coding guidelines, errors should not be ignored with `_` without justification. While test cleanup errors are often tolerable, logging them helps diagnose intermittent test failures.

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. Both tests in this file defer `db.Close()` and discard the returned error with `_`, which conflicts with the repository guidance against ignoring errors without justification.
  - Root cause: the new test file copied the existing `openTestGlobalDB` cleanup pattern without surfacing teardown failures, so intermittent SQLite close errors would be hidden.
  - Fix approach: keep the cleanup local to this file but report close failures via the test handle instead of silently discarding them.

## Resolution

- Updated both deferred cleanups in `internal/store/globaldb/read_queries_test.go` to report `db.Close()` failures through `t.Errorf(...)`.
- Verification:
- `go test ./internal/store/globaldb ./pkg/compozy/runs -count=1`
- `make verify`
