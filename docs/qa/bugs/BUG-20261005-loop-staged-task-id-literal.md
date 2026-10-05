# BUG-20261005-loop-staged-task-id-literal: Staged task outputs retain the template instead of the task ID

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-01, inspect a staged task during orchestrated delivery
- **Scenarios:** LP-implement-tasks-orchestrated-mode; TA-080
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The bundled implement-tasks Loop stages three authored tasks correctly for its conductor,
but every stage_orchestrated output identifies the task as {{ .item.id }}. Bruno cannot
use that result to identify the staged task. Delivery continues; no task-loss claim is made.

## Reproduction

- **Charter:** CH-implement-tasks-orchestrated-mode · **Tour:** Feature Tour
- **Environment:** 1512x862, DPR 2, fast Wi-Fi, en-US, America/Los_Angeles

1. Author three dependent tasks and start the bundled implement-tasks Loop with mode=orchestrated.
2. Read the Run with the scoped loop status CLI.
3. Open the Run in Web and choose Details for stage_orchestrated (round 1, item 1).
4. Reload the page and reopen that output.

**Expected:** task_id names task_01, task_02 or task_03 for the corresponding item.
**Actual:** every persisted result contains the literal template {{ .item.id }}.

## Evidence

- Run: looprun-741e0ce2ba97875f
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-status-scoped.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-stage-output-web.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-stage-output-reload.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-stage-output-reloaded.png

## Investigation

The public importer returns concrete task identities and the three staging cells retain
the same placeholder. Root-cause investigation and repair follow the live delivery walk.
No runtime or definition was changed during the failing observation.

The runtime transform contract treats value as literal and from as a namespace reference.
The bundled definition incorrectly authors the item template under value. The correction
belongs to that definition, preserving literal transform semantics for existing users.

## Fix

- **Fix commit:** pending
- **Regression test:** existing TestEmbeddedLoopsShouldKeepSpecCycleRuntimeContracts,
  executing the shipped transform for three distinct task identities.
- The bundled YAML and its copyable site example use from: item.id. No transform engine
  semantics, public schema or historical output is rewritten.

## Verification

The owning regression fails before repair and passes with -race. Fresh real-provider Run
looprun-61931098226ef84d stages task_01 and task_02, completes both tasks and settles done.
CLI, HTTP/UDS and the Web Details view agree; the concrete first ID survives a reload.
Both workers are independently stopped / verified=true and the active child list is empty.
Evidence: loops-orchestrated-regressions-{red,green}.json;
loops-orchestrated-fixed-{status-third,staged-web,final-http,final-uds}.json;
loops-orchestrated-fixed-staged-id-reloaded.png. The final delivery commit is pending.
