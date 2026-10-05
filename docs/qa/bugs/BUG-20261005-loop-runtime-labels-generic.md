# BUG-20261005-loop-runtime-labels-generic: Loop runtime selectors hide their field names

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea; people navigating by accessible control name
- **Journey Step:** J-01, choose runtime inputs before starting a Loop
- **Scenarios:** LP-002
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The four runtime selectors in the implement-tasks form all announce the same generic
Runtime name. A person navigating by accessible name cannot distinguish the backend,
frontend, default and orchestrator fields, even though their visible captions differ.

## Reproduction

- **Charter:** CH-001 · **Tour:** Feature Tour
- **Environment:** laptop 1512×862, DPR 2, en-US, Wi-Fi; Studio Operations / resume-editorial

1. Open Loops from the dock, then implement-tasks and Run loop.
2. Inspect or navigate the four optional runtime controls by their accessible names.

**Expected:** Each control announces its visible caption and current runtime selection.
**Actual:** All four announce Runtime: / Select model. Their correct native label sources
are superseded by the selector's generic aria-label.

## Evidence

- docs/qa/evidence/2026-10-02-untested/loops-first-run-lea-input-controls.json
- docs/qa/evidence/2026-10-02-untested/loops-first-run-lea-empty-form-limits.png
- The actual run succeeds; this finding concerns field identification, not execution.

## Fix

- **Root cause:** LoopRuntimeValueControl explicitly passes an undefined ariaLabelledby.
  The Loop field wrappers do not forward a caption identity, so RuntimeSelector uses its
  standalone generic name, overriding each native label association.
- **Scope:** Wire caption IDs through the existing typed-control composition for both run
  and automation forms. Reuse RuntimeSelector's existing caption-plus-value behavior.
  No migration, public API change or product trade-off is involved.
- **Fix commit:** 60ddd98e12e10731a7f98d8dedca20ff88f2e459
- **Regression test:** Existing loop-run-input-field.test.tsx runtime case distinguishes
  two fields with the same selected model by their captions and still changes runtime speed.

## Verification

- **Retested:** 2026-10-05, Lea / J-01, same report. All four run inputs expose their distinct
  captions. Selecting Codex / GPT-6.1-Sol preserves Backend runtime in the name. The Add
  schedule form also exposes four distinct caption-plus-value names. Cancel and reload leave
  no job; the canonical UDS jobs catalog independently returns total 0.
- **Evidence:** loop-runtime-labels-lea-{form,automation-entry,job-observed,finish,
  no-job-canonical}.json; form/automation/abandoned PNGs under the report evidence directory.
  The exact loop-runtime-labels-lea recording is closed (16 frames). Build, lint and typecheck
  pass. The bug stays open until its fix commit is recorded; the full LP-002 session picker
  leg is still pending.


The full LP-002 older-session continuation also passes on the final catalog bundle.
The original label fix is unchanged; all four named controls remain visible in the
copied run form. The 22-frame loops-picker-lea recording is closed, its dry run creates
no run, and every temporary definition/session is removed with baseline equality.


## Verified delivery — 2026-10-05

Fix commit: 60ddd98e12e10731a7f98d8dedca20ff88f2e459. Original-persona replay and the owning checks pass.
The warning-free affected gate passes, and the commit tree exactly matches
f1493b4c0a97be585cc4fe2e60b941df61a20ba4. Receipts: qa-catalog-delivery-gate-6.json
and qa-catalog-repairs-commit-identity.json in docs/qa/evidence/2026-10-02-untested/.
