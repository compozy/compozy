---
provider: coderabbit
pr: "201"
round: 3
round_created_at: 2026-06-15T21:50:39.00729Z
status: resolved
file: internal/core/run/ui/update.go
line: 60
severity: major
author: coderabbitai[bot]
provider_ref: review:4501451317,nitpick_hash:e36e97f5e9b1
review_hash: e36e97f5e9b1
source_review_id: "4501451317"
source_review_submitted_at: "2026-06-15T21:50:09Z"
---

# Issue 001: Route runStatusMsg through single-message dispatch or it gets dropped.
## Review Comment

`applyUIMsg` now supports `runStatusMsg`, but `dispatchSingleUIMsg` does not. Since remote bootstrap enqueues messages individually (`session.Enqueue(msg)`), `runStatusMsg` can be ignored in the default `Update` path, so `m.runStatus` never updates from those events.

Also applies to: 127-129

<!-- cr-comment:v1:49c0077049d9f1549f5920cf -->

## Triage

- Decision: `VALID`
- Root cause: `remoteSnapshotBootstrap` emits `runStatusMsg` as the first snapshot bootstrap message and
  `AttachRemote` enqueues each bootstrap message individually with `session.Enqueue(msg)`. The normal
  Bubble Tea `Update` path delegates individual UI messages through `dispatchSingleUIMsg`, but that switch
  does not include `runStatusMsg`. Batch dispatch does call `applyUIMsg`, which already handles
  `runStatusMsg`, so the two dispatch paths disagree and individually enqueued run status updates are
  dropped.
- Intended fix: add `runStatusMsg` to the explicit single-message dispatch allow-list and add focused
  regression coverage that calls `Update(runStatusMsg{...})` so the canonical single-message path is
  exercised instead of only testing `applyUIMsg` directly.
- Resolution: added the missing `runStatusMsg` branch in `dispatchSingleUIMsg` and covered it with
  `TestUpdateDispatchesSingleRunStatusMsg`, which verifies an individually delivered run status message
  updates `m.runStatus` through `uiModel.Update`.
- Verification: `rtk go test ./internal/core/run/ui -run TestUpdateDispatchesSingleRunStatusMsg -count=1`
  passed. Initial full gate `rtk make verify` completed with exit code 0 before this issue file was marked
  resolved. Final pre-commit gate `rtk make verify` completed with exit code 0 after this issue file was
  marked resolved.
