# BUG-20260828-session-runtime-restart-projection: Restart drops the public runtime selection

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Trust-Damage
- **Severity:** Major · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-17 Reopen a stopped session after daemon restart
- **Scenarios:** RT-session-runtime-selection-continuity
- **Found:** 2026-08-28 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

The durable session metadata retained the selected logical runtime, typed ACP options, revision, generation, and recovery state, but restart reconciliation and durable catalog reads omitted those fields from the public session projection.

## Fix

- **Root cause:** both restart reconciliation and the durable list projection copied lifecycle state without copying the runtime preference and recovery fields stored beside it.
- **Fix:** project the complete runtime metadata through reconciliation and durable session listing.
- **Fix commit:** pending; included in the single remediation commit
- **Regression tests:** `TestReconcileActiveSessions`; `TestManagerSessionQueries` in the canonical observe and session query suites.

## Verification

- **Focused automated result:** the observe reconciliation and session query suites pass with `-race`.
- **Restart result:** after a real daemon restart, stopped session `sess-106eb8fa23dfb9c0` still reports Cursor, `grok-4.6`, `xhigh`, Fast, selection revision 2, and runtime generation 1.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/session-runtime-restart-projection.json`.
