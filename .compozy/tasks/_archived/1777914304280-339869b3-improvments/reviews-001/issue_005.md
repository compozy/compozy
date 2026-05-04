---
status: resolved
file: internal/api/contract/contract_test.go
line: 67
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:429125d1da21
review_hash: 429125d1da21
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 005: Use Should... phrasing for the new table test case names.
## Review Comment

Please align these two entries with the required test-case naming pattern used by `t.Run`.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

## Triage

- Decision: `VALID`
- Notes: The added timeout policy subtests used non-`Should...` names. Renamed the table entries to the repository's required `Should...` pattern while preserving assertions.
