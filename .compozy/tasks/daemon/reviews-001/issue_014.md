---
status: resolved
file: internal/api/httpapi/transport_integration_test.go
line: 487
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:af073e280238
review_hash: af073e280238
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 014: Rename these subtests to the required Should... form.
## Review Comment

`"invalid"` and `"stale"` make failures harder to scan, and they don't match the repo's required subtest pattern.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

## Triage

- Decision: `VALID`
- Root cause: the subtests in `TestHTTPStreamRejectsInvalidAndStaleCursor` use terse names that do not follow the repo’s `Should...` naming rule.
- Fix plan: rename the subtests to descriptive `Should...` forms.
- Resolution: Implemented and verified with `make verify`.
