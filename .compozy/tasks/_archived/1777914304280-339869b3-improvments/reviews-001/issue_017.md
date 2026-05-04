---
status: resolved
file: internal/core/agent/registry_test.go
line: 443
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:8c98c098fa0f
review_hash: 8c98c098fa0f
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 017: Consider adding t.Parallel() for test isolation.
## Review Comment

This test modifies `PATH` via `t.Setenv()`, which is scoped to the test. Adding `t.Parallel()` would allow it to run concurrently with other tests that also use isolated `PATH` modifications.

## Triage

- Decision: `INVALID`
- Notes: The target test uses `t.Setenv`, and Go's testing package forbids `t.Setenv` in parallel tests or tests with parallel ancestors because environment mutation is process-wide. Adding `t.Parallel()` would make this test invalid rather than improve isolation, so no code change was made for this issue.
