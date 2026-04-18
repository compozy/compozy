---
status: resolved
file: internal/cli/daemon_commands_test.go
line: 414
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:fb15a862609f
review_hash: fb15a862609f
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 022: Complex ref-counting lock pattern for global test overrides.
## Review Comment

This implements a ref-counting scheme to serialize tests that modify global state (`newCLIDaemonBootstrap`). The pattern:
1. Check if this test already has refs (reentrant case)
2. If not, acquire the global mutex `cliTestGlobalOverrideMu`
3. Track refs per test name
4. Release global mutex only when last ref is released

The nested locking is correct but complex. Consider adding a brief comment explaining why ref-counting is needed (likely for subtests that install multiple overrides).

## Triage

- Decision: `invalid`
- Why: this is a readability suggestion, not a correctness defect. The ref-counted lock is already scoped to the test helper, its behavior is straightforward once read with the paired release path, and there is no production or test regression to fix here.
- Notes: closed as analysis-only for this batch. No code or test change is required to satisfy the execution contract. Repository verification still passed on the final batch state via `make verify`.
