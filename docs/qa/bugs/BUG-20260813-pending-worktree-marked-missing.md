# BUG-20260813-pending-worktree-marked-missing: Catalog refresh corrupts an accepted creation

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Operators creating a worktree in the Web app
- **Journey Step:** Workspace menu → New worktree → Create worktree
- **Scenarios:** RT-worktree-web-create-adopt; RT-worktree-web-nested-navigation
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md
- **Origin:** Task 10 release QA

## Summary

The Web create request returned an accepted `pending` row, but the immediate catalog refresh saw
that its checkout directory did not exist yet and changed it to `missing`. The asynchronous creator
then could not complete the pending row, leaving a branch without a checkout and a permanently
missing record.

## Reproduction

1. Open the desktop workspace menu and choose **New worktree**.
2. Submit a name while the worktree catalog stream is live.
3. Observe the accepted row become `missing` during its normal `branch` phase.

## Fix

Missing-path reconciliation now applies only to stable `ready` rows. Transitional and failed rows
retain their domain state until their owning operation changes it.

Regression suite: `internal/worktree.TestServiceDiscovery`.

- **Fix commit:** `b6eb94d0`

## Verification

- `go test -race ./internal/worktree -count=1`: 196 passed.
- Strict scoped Go lint and the production build passed.
- A clean browser replay removed the pending affordance without intervention; the public API and
  `git worktree list` both reported the checkout as ready.
- Evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.json` and
  `browser-worktree-stream-proof.png`.
