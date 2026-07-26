# BUG-20260715-task-run-designation-hidden: Run detail hides coordinator designation

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-enable-coordinated-conversations, open an eligible coordinator run
- **Scenarios:** NB-coordination-invitation-future-runs
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Task runs stored designation-group metadata, but the public task-run payload dropped the parsed designation summary. The Web route therefore could not identify index-zero coordinator runs and suppressed the contextual future-coordination invitation even when the persisted run was eligible.

## Reproduction

- **Charter:** CH-coordination-future-runs · **Tour:** Back-Button Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Fan out one active Local task into a coordinator and worker.
2. Read the coordinator run through the public task-run route.
3. Open the same run in Web.

**Expected:** The payload exposes designation index zero and the eligible coordinator invitation renders.
**Actual:** `designation_group_id` existed, but `designation` was absent and Web treated the coordinator as an ordinary run.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md`
- Live retest: coordinator `run-b33e50a8f7706eb6` exposed index zero, rendered the invitation, and supported durable dismissal and task-scoped acceptance.

## Fix

- **Root cause:** `TaskRunPayloadFromRun` copied the group ID and raw metadata but never converted `DesignationFromRun` into the public summary.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical API task payload and expanded surface suite require the designation group, index, and brief to survive public conversion.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** the real coordinator invitation rendered; dismissal persisted after refresh; acceptance advanced the selected task scope once without mutating existing Local runs.
