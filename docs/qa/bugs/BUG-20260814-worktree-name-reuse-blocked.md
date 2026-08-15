# BUG-20260814-worktree-name-reuse-blocked: Default recreation conflicts with an intentionally retained branch

- **Status:** invalid
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-worktree-management, step 7
- **Scenarios:** RT-worktree-cli-lifecycle
- **Found:** 2026-08-14 · **Report:** docs/qa/reports/2026-08-14-worktree-lifecycle-fixes.md

## Summary

The first QA command treated the retained Git branch as an unexpected reservation. The product deliberately removes the checkout while preserving branch history; dismissal frees the catalog name, not the independent Git branch name. This finding is therefore not a product bug.

## Reproduction

- **Charter:** CH-worktree-lifecycle-surface-parity · **Tour:** Feature Tour
- **Environment:** desktop / 1280×800 / wifi-fast / en-US

1. Create `feature-analytics` from the Web and wait for it to become ready.
2. Run `compozy worktree remove feature-analytics --workspace acme-dashboard --force --json`.
3. Run `compozy worktree dismiss feature-analytics --workspace acme-dashboard --json`.
4. Run `compozy worktree create feature-analytics --workspace acme-dashboard --base main --json`.

**Expected:** The catalog name is available. When a Git branch with the derived name remains, the operator chooses it explicitly with `--existing-branch feature-analytics` or supplies a different new branch.
**Actual:** The original command correctly returned `branch_held_by_worktree`; the corrected command using the retained branch created a new pending identity that reached `ready`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-recreate-same-name.json`
- Independent Git read: the main checkout is the only linked worktree, but branch `feature-analytics` remains.

## Fix

- **Root cause:** Invalid QA expectation. Worktree removal preserves Git branches and their commit history by contract; catalog dismissal does not delete Git data.
- **Fix commit:** not applicable
- **Regression test:** The canonical daemon lifecycle integration now recreates the dismissed catalog name with the retained branch explicitly, while storage coverage independently proves that a dismissed row no longer reserves its catalog name.

## Verification

- **Retested:** 2026-08-14 with `compozy worktree create feature-analytics --existing-branch feature-analytics` after remove and dismiss.
- **Result:** Invalid finding. Recreation succeeded as `wt_0d43db3f172e7b79`, reached `ready`, name lookup selected the new row, and the old dismissed row remained readable by ID.

Corrected evidence:

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-recreate-same-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-new-name-ready.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-old-id-tombstone.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/teardown.json` (`clean: true`)
