# BUG-20260814-worktree-mutation-output-loses-identity: Name-addressed mutations return an empty worktree identity

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-worktree-management, step 7
- **Scenarios:** RT-worktree-cli-lifecycle
- **Found:** 2026-08-14 · **Report:** docs/qa/reports/2026-08-14-worktree-lifecycle-fixes.md

## Summary

Removal and dismissal by name complete, but their structured output replaces the canonical worktree identity with the entered name and empty fields, so an operator cannot trust or correlate the result.

## Reproduction

- **Charter:** CH-worktree-lifecycle-surface-parity · **Tour:** Feature Tour
- **Environment:** desktop / 1280×800 / wifi-fast / en-US

1. Create and inspect `feature-analytics`; record canonical id `wt_281e7b20dc99dea1`.
2. Remove it by name with structured JSON output.
3. Dismiss it by name with structured JSON output.

**Expected:** Both results identify the same canonical row and its terminal state.
**Actual:** Both return `worktree.id: "feature-analytics"`, empty workspace/name/path/state fields, and zero timestamps.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-remove-by-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-dismiss-by-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-inspect-by-name.json`

## Fix

- **Root cause:** The CLI passed the entered reference to a mutation endpoint that correctly returned no body, then fabricated a `WorktreeRecord` with that reference as its ID. The status projection and native removal path also failed to carry the canonical row ID through their boundary payloads.
- **Fix commit:** working tree
- **Regression test:** `internal/cli/worktree_test.go` now crosses names through cancel, remove, dismiss, and exit-cancel and asserts canonical receipts; `internal/api/testutil/worktree_transport_parity_test.go`, `internal/daemon/native_worktree_tools_test.go`, and `internal/worktree/status_test.go` own transport and status identity parity.

## Verification

- **Retested:** 2026-08-14 in the same isolated workspace after rebuilding and restarting the daemon.
- **Result:** Verified. Name-addressed status, removal, and dismissal all reported `wt_1abea7281257b9ce`; removal returned `state: removed`, dismissal returned `state: dismissed`, the old row remained readable by that ID, and recreation returned the distinct ready identity `wt_0d43db3f172e7b79`.

Retest evidence:

- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-status-by-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-remove-by-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-dismiss-by-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-old-id-tombstone.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-recreate-same-name.json`
- `/Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/teardown.json` (`clean: true`)
