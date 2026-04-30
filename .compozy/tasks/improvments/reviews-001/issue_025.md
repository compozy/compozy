---
status: resolved
file: internal/core/run/executor/execution_test.go
line: 317
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:555a18fca644
review_hash: 555a18fca644
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 025: Scope the empty-pr assertion to frontmatter only.
## Review Comment

Checking the full markdown content can false-fail if `pr:` appears in body text; limiting the check to frontmatter makes this test more robust.

## Triage

- Decision: `valid`
- Notes: Confirmed the assertion searched the full markdown content for `pr:` and could fail if the body text mentioned that key. Scoped the check to the frontmatter block only.
