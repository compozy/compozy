# BUG-20260901-filtered-fanout-phantom-rows: Successful filtered fan-out looks unfinished

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-partial-loop, step 3
- **Scenarios:** LP-fan-out-filtering; LP-run-read-agent-journey; LP-runs-roster-server-ordering; LP-web-run-operator-register
- **Found:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-issue-506-filtered-fanout.md
- **Origin:**

## Summary

A filtered fan-out can finish successfully while the roster invents pending workers for source
positions rejected by the filter. Progress, rollups, run summaries, and the Web Inspect view then
show unfinished work that never existed.

## Reproduction

- **Charter:** CH-loop-graph-runtime-safety · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US

1. Validate and run a fan-out over three source items with a filter that retains only source index 2.
2. Wait for the run to succeed.
3. Read the roster and fan-out rollup through the CLI, then compare HTTP, UDS, native-tool, and Web reads.

**Expected:** The roster contains only worker index 2 and reports progress `1/1`.
**Actual:** The roster contains indexes 0, 1, and 2, leaves 0 and 1 pending, and reports progress `1/3`.

## Evidence

- GitHub issue `compozy/compozy#506` records the public-surface reproduction.
- `go test -race ./internal/loop -run '^TestRosterContract$/^Should_preserve_sparse_fanout_item_indexes_without_inventing_pending_rows$' -count=1` failed with indexes `[0 1 2 3 4 5]` instead of `[2 5]` before the fix.

## Fix

- **Root cause:** The roster indexed outputs by their maximum item index, then projected every integer from zero through that maximum instead of the exact persisted identities.
- **Fix commit:** e96962c
- **Regression test:** `internal/loop.TestRosterContract/Should preserve only materialized fanout item indexes` failed before the production change and passes after it.

## Verification

- **Retested:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-issue-506-filtered-fanout.md
- **Result:** Passed. A one-worker run exposed only index `2`; a second run exposed only indexes
  `2` and `4`. CLI, HTTP, UDS, and native reads agreed, and Web Inspect showed `2 of 2 done`
  before and after reload. Evidence is indexed from the report.
