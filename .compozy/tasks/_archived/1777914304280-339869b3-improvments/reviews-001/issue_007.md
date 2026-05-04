---
status: resolved
file: internal/api/core/handlers_smoke_test.go
line: 363
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:3a2a5ed85694
review_hash: 3a2a5ed85694
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 007: Rename the new subtest case to the required Should... format.
## Review Comment

Since this table name is passed into `t.Run`, please rename `"run transcript"` to a `Should ...` phrase for consistency with the enforced test pattern.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

## Triage

- Decision: `VALID`
- Notes: The smoke test table used `run transcript` as a `t.Run` name. Renamed it to `Should serve run transcript`.
