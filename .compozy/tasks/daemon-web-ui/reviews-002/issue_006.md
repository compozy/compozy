---
status: resolved
file: internal/core/extension/runtime_test.go
line: 350
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:c3426cb3263f
review_hash: c3426cb3263f
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 006: No current deadlock risk detected, but consider consolidating HOME isolation to prevent future double-acquisition.
## Review Comment

Verification confirms no test function currently calls `runtimeConfigForTest(t)` multiple times or mixes it with `isolateRunScopeHome(t)`. However, since `isolateRunScopeHome(t)` acquires the non-reentrant `runScopeTestHomeMu` (line 368), accidental double calls would deadlock until cleanup. To improve API safety and prevent future mistakes, consider either making HOME isolation idempotent per test (e.g., check if already locked) or consolidating acquisition at one explicit layer so the risk is eliminated by design.

## Triage

- Decision: `invalid`
- Notes:
  - Current call-site inspection shows each test invokes `runtimeConfigForTest(t)` at most once, and no test mixes that helper with a separate direct `isolateRunScopeHome(t)` call.
  - The review comment describes a hypothetical future misuse, not a bug or regression in the current code.
  - Analysis complete: no code change was made because adding re-entrant bookkeeping would increase test-only complexity without addressing a present failure mode.
