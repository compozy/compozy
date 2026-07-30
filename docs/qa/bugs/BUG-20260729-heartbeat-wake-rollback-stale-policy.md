# BUG-20260729-heartbeat-wake-rollback-stale-policy: Wake ignored the policy restored by rollback

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-31, restore HEARTBEAT.md and dry-run Wake
- **Scenarios:** RT-077
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated HTTP and UDS replay

## Summary

Managed rollback restored the v1 HEARTBEAT.md file, status, digest, and history revision, but the next
Wake decision still used v2. A restored policy therefore appeared correct in every authoring surface
while runtime execution followed stale guidance.

## Reproduction

1. Save Heartbeat policies v1 and v2 for one workspace agent.
2. Roll back v2 to v1 and confirm the current route and newest revision report v1.
3. Execute a dry-run Wake for an eligible session over HTTP or UDS.

**Expected:** Wake uses the newest authoring revision's v1 snapshot and digest.
**Actual before the fix:** Wake returned v2 snapshot `hb-149ac58623488d47` and digest
`sha256:9469a792a50bf53a5a77d2fa6eae19ee1dfb76522dfba88ecd588c999be13843`.

## Evidence

- Lifecycle, rollback, pre-fix decision, repaired parity, and cleanup assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files`.
- The repaired HTTP and UDS decisions matched byte-for-byte and selected v1 snapshot
  `hb-4337d2bcdfedde3a` with digest
  `sha256:4a17d89d1103ffc73c8fcd178215973ef94f6b82d1fbdc784b752517fa3a8167`.

## Fix

- **Root cause:** snapshot rows are immutable and deduplicated by digest, so rollback correctly reused
  the older v1 row. `GetLatestValidHeartbeatSnapshot` incorrectly treated snapshot `created_at` as the
  activation order and selected the newer-created v2 row.
- **Correction:** managed authoring revisions are now the current-policy activation ledger. Wake follows
  the newest revision's `NewSnapshotID`; direct snapshot ordering remains only when no revision exists.
- **Fix commit:** `351f3535`
- **Regression test:** `Should use the latest rollback revision as the current wake policy` in
  `internal/store/globaldb/global_db_heartbeat_test.go`.

## Verification

- The canonical GlobalDB regression failed red with `hb-second`, then passed with the rollback target
  `hb-first`.
- The rebuilt live daemon returned exact v1 HTTP/UDS parity and left ACP diagnostics unchanged at 28
  lines during both dry runs.
- The full GlobalDB lane's unrelated bridge-migration case exceeded its 50-second deadline only under
  suite saturation; that exact migration case passed in isolation in 13.9 seconds.
- `make lint` and `make build` pass.
- **Retested:** 2026-07-29, rebuilt daemon candidate green; fix shipped in `351f3535`
