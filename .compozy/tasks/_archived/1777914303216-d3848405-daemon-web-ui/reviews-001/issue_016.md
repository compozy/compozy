---
status: resolved
file: internal/daemon/run_snapshot_test.go
line: 14
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:9b319467a3ad
review_hash: 9b319467a3ad
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 016: Consider breaking helper assertions into subtests for clearer test output.
## Review Comment

The test covers the snapshot builder lifecycle comprehensively, but the inline helper function tests at lines 205-231 (`cloneRunJobSummary`, `tokenUsageRowToKinds`, `tokenUsageIndex`, `shutdownStateFromPayload`) would benefit from `t.Run("Should...")` subtests per coding guidelines. This improves failure isolation and test output readability.

## Triage

- Decision: `valid`
- Notes:
  - The lifecycle test ends with several unrelated helper assertions in a single flow, which weakens failure isolation and diverges from the repository’s subtest pattern.
  - Root cause: helper-behavior checks were appended to the main lifecycle scenario instead of being named subtests.
  - Intended fix: keep the main lifecycle coverage intact and move the helper assertions into explicit `t.Run("Should ...")` subtests.

## Resolution

- Broke the helper assertions in `run_snapshot_test.go` into named `Should ...` subtests while keeping the main lifecycle test intact.
- Verified with `make verify`.
