---
status: resolved
file: internal/daemon/boot_integration_test.go
line: 327
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149167497,nitpick_hash:c48e7fe71cc4
review_hash: c48e7fe71cc4
source_review_id: "4149167497"
source_review_submitted_at: "2026-04-21T16:03:49Z"
---

# Issue 003: Prefer a table-driven shape for the run-mode assertions.
## Review Comment

The foreground and detached branches share the same harness and differ only by mode plus stderr expectations. Folding them into a small table will keep future mode coverage in sync and matches the repo’s default testing pattern.

As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern".

## Triage

- Decision: `INVALID`
- Reasoning: `TestManagedDaemonRunModesControlLogging` already uses explicit `t.Run(...)` subtests for the two run modes, and there is no divergence bug or missing coverage caused by the current shape.
- Root cause: The review requests a stylistic table refactor, not a defect fix.
- Resolution plan: No code change. Keep the existing subtests because they are already clear and in line with the repository's default testing guidance.

## Resolution

- Closed as `invalid`. The current subtest structure already covers the two run modes clearly, so no table refactor was necessary.

## Verification

- Confirmed against the current test file and completed a fresh `make verify` pass after the in-scope fixes for the valid issues.
