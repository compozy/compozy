# BUG-20260813-base-ref-accepted-before-validation: Invalid base leaves create UI pending

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-worktree-management, create
- **Scenarios:** RT-worktree-web-create-adopt; RT-worktree-api-surface-parity
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

Creating a worktree from a missing base ref was accepted as pending before Git validated the ref.
The asynchronous worker then deleted the row, leaving the Web dialog in `materializing` and making
its cancellation fail with `worktree_not_found`.

## Reproduction

- **Charter:** CH-worktree-lifecycle-surface-parity · **Tour:** Feature Tour
- **Environment:** macOS arm64, isolated runtime, en-US

1. Open **New worktree** and expand **Advanced**.
2. Enter an unknown value in **Base ref** and submit.
3. Try to cancel the accepted creation.

**Expected:** The request is refused synchronously as `base_ref_not_found`, the error lands on
**Base ref**, and no pending record is created.
**Actual:** HTTP returned `202`; the dialog showed `materializing`; cancellation returned
`worktree_not_found` after the worker deleted the record.

## Fix

- **Root cause:** Branch/base identity resolution ran inside asynchronous materialization, after
  pending persistence and acceptance.
- **Fix commit:** `0d54b6fe`
- **Regression test:** `internal/worktree/create_test.go` — `TestServiceCreate`, missing refs case

## Verification

- **Retested:** 2026-08-13 in
  `compozy-worktree-support-20260813-083057-155448-lab`.
- **Result:** Passed. HTTP returned `409` with `code=base_ref_not_found`, no worktree row was created,
  and the Web dialog rendered **The base ref could not be found.** on the owning field.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/base-ref-refusal-fixed.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-base-ref-refusal-fixed.png`.
