---
status: resolved
file: packages/ui/src/tokens.css
line: 2
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:50e4f85ed170
review_hash: 50e4f85ed170
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 025: Fix current Stylelint violations in this file.
## Review Comment

These flagged issues are straightforward and will keep lint green: unquoted single-word font family, keyword-case warnings in font stack, missing empty line, and `text-rendering` value case.

Also applies to: 93-93, 232-233

## Triage

- Decision: `invalid`
- Notes:
  - The comment is anchored to Stylelint-only findings, but the current repository does not run Stylelint in scripts or CI, and the cited line numbers no longer match the current `tokens.css` after the in-flight typography/token rewrite.
  - Root cause of the comment: it was generated against an earlier snapshot of the file and against tooling that is not active in this branch.
  - Resolution path: no change for this batch. If repository verification later reports a real CSS failure, address that concrete signal instead of preemptively editing for a non-existent linter.

## Resolution

- Closed as invalid. The cited Stylelint findings do not correspond to an active verifier or to the current file shape in this branch.
- Verification:
- `make verify`
