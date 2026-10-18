# BUG-20261004-automation-locked-target-repair-copy: Blocked edit suggests a locked target picker

- **Status:** fixed — verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora
- **Journey Step:** J-24, edit a saved automation whose Loop no longer accepts its start kind
- **Scenarios:** TA-web-automation-preview-toggle
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In the isolated profile-recovery lab, create a Loop accepting schedule and trigger starts,
create a disabled Job and Trigger bound to it, then publish a new Loop version without those
starts. All preparation uses the public CLI and HTTP surfaces. As Dora, open either saved
automation's editor through its catalog and detail. The warning correctly explains the missing
start kind but says to choose a compatible Loop. The existing Loop picker is disabled in edit
mode, so that instruction cannot be followed. Preview repeats the same instruction.

**Expected:** explain how to restore the saved target's compatibility or availability.
**Actual:** suggest an action reserved for creation, while the saved target remains immutable.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:

- automation-preview-replay-blocked-targets.json (disabled Job controls)
- automation-preview-replay-blocked-targets-observed.json (Job preview)
- automation-preview-replay-trigger-and-directions.json (Trigger form, controls and preview)
- automation-preview-replay-job-target-warning.png
- automation-preview-replay-trigger-target-warning.png
- automation-preview-replay-directions-ended.json

The two warning screenshots were inspected. The fresh Dora Garbage Tour recording stopped
with 53 frames at /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/automation-preview-replay-dora.

## Fix

- **Root cause:** loopTargetAvailabilityMessage distinguishes create/edit only by a retained
  selection suffix, then gives both modes the creation-only target replacement instruction.
- **Repair:** edit mode names restoring the Loop's start kind or access. Creation retains the
  available picker remedy. Existing selection, validation, disabled primary and payload remain.
- **Owning layer:** shared Loop target availability presentation, consumed by both form and preview.
- **Regression proof:** real before/after Dora replay; existing job/trigger form suites own
  preserved target and blocked submission. No prose-only automated assertion is added.
- **Fix commit:** a2917318c96359b507b0e0a0fc06ad410f7dd648.

## Verification

Fresh Dora replay verifies incompatible and unavailable targets in both editors, form and
preview. Saving stays blocked; immutable targets and drafts survive, and Cancel preserves the
saved definitions as independently confirmed by HTTP readback. Four warning screenshots were
inspected. Evidence: automation-preview-final-dora-ended.json, the final round-trip receipts
and automation-preview-final-readback-*.json. The affected gate passes; fix SHA: a2917318c96359b507b0e0a0fc06ad410f7dd648.
