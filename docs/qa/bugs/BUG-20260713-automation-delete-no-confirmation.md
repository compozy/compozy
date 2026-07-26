# BUG-20260713-automation-delete-no-confirmation: Dynamic automations delete immediately without confirmation

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24 delete one dynamic Job or Trigger
- **Scenarios:** TA-automation-crud-loop-target
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

The Job detail Delete action is an immediate destructive mutation. One click removed `software-delivery-qa`, including its schedule and run-history access point, without a confirmation dialog, named target context, explicit destructive confirmation, or undo. Trigger detail uses the same direct-delete interaction.

## Reproduction

1. Create and run a workspace-scoped dynamic Job.
2. Open its detail page and choose Delete once.
3. Observe that the Job disappears immediately and the catalog reports zero Jobs.

**Expected:** A destructive confirmation modal identifies the selected automation, Cancel preserves it, and an explicit Delete confirmation removes only that dynamic definition.
**Actual:** The first click performs the irreversible deletion with no confirmation state.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-job-delete-no-confirmation-before.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-job-delete-no-confirmation-after.dom.txt`
- The before DOM contains one enabled `Delete` button; the immediately following DOM contains `0 jobs found` and no dialog.

## Fix

- **Root cause:** The dynamic automation detail action invokes the delete mutation directly instead of owning a confirmation lifecycle.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** The canonical Job and Trigger detail suites own workspace-only visibility, typed-name gating, Cancel, one-shot confirmation, success navigation/cache invalidation, and failure preservation.

## Verification

- Same-persona browser replay passed for both object types. Job Delete opened a named dialog; Cancel and an incorrect name preserved `software-delivery-qa`; exact-name confirmation removed it once and a fresh catalog showed zero Jobs. Trigger Delete repeated the same safety lifecycle after its exactly-once delegated run, and a fresh catalog showed zero Triggers.
