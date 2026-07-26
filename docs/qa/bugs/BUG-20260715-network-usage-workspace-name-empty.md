# BUG-20260715-network-usage-workspace-name-empty: Named workspace usage appears empty

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-run-bounded-live-collaboration, reconcile workspace usage
- **Scenarios:** NB-run-bounded-live-collaboration; NB-agent-manages-participation
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Ada queried Network usage with the registered workspace name and received an empty report even though the same isolated workspace had settled wakes. Name-addressed CLI and UDS reads therefore contradicted the durable usage ledger.

## Reproduction

- **Charter:** CH-live-bounds-agent-path · **Tour:** Interrupt Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Register workspace `bounds-lab` and settle at least one Live wake.
2. Run `agh network usage --workspace bounds-lab -o json`.
3. Compare with the same route addressed by canonical workspace ID.

**Expected:** Both addresses return the same canonical workspace-scoped usage.
**Actual:** The name-addressed route originally returned no details because the store query received `bounds-lab` instead of `ws_b94c43d66a542d79`.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md`
- Live CLI and UDS retest both returned eight wake rows under canonical ID `ws_b94c43d66a542d79`.

## Fix

- **Root cause:** The shared API route resolver verified the workspace name but returned the raw route token instead of the resolved workspace ID.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `internal/api/core/network_usage_public_test.go` routes through workspace name `alpha`, requires store query `ws-alpha`, and asserts the canonical response identity.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** CLI and UDS name-addressed reads return the complete canonical usage report.
