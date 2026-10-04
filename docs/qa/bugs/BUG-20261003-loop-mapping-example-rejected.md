# BUG-20261003-loop-mapping-example-rejected: The Loop mapping example is rejected on submit

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-24, map an incoming webhook field to a Loop input
- **Scenarios:** ET-web-jobs-triggers-catalog
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno follows the mapping field's example, enters trigger.payload.edition, and cannot save the
Trigger until a server error teaches the different expression syntax it accepts.

## Reproduction

- **Charter:** CH-trigger-detail-rule-page · **Tour:** Feature Tour
- **Environment:** Chrome, desktop 1512x862, wifi-fast, en-US

1. Enter Triggers from the Dock and create an incoming webhook targeting a compatible Loop.
2. Supply the required endpoint, signing secret and Loop input.
3. Follow the mapping placeholder trigger.payload.field, replacing field with edition.
4. Submit from the live preview.

**Expected:** The field's example uses an expression accepted by the existing server contract.
**Actual:** Submission returns start_input_mapping_invalid; it requires {{ .trigger.payload.<field> }}.
Replacing the expression with {{ .trigger.payload.edition }} saves the Trigger and a real delivery
maps the value into a completed Loop run.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: trigger-preview-error-bruno-corrected-result.json,
trigger-preview-error-bruno-recovery.json, trigger-preview-error-bruno-saved-readback.json,
trigger-rule-bruno-webhook-delivery.json and trigger-rule-bruno-open-loop.json.

## Fix

- **Root cause:** LoopInputMapping hard-codes a bare path as its placeholder and comment,
  although the existing mapping grammar requires a braced template expression.
- **Fix commit:** self (the commit that records this verified repair)
- **Regression test:** A fresh rendered-form replay will follow the corrected example through
  save and independent readback. A prose-only placeholder assertion would freeze copy without
  proving the accepted mapping, so no new automated test is required for this copy repair.

## Verification

- **Retested:** 2026-10-03, Bruno, CH-trigger-detail-rule-page.
- **Result:** Fresh Bruno edit follows the corrected example, saves February publication and the edition mapping, then confirms both through reload and UDS. Evidence: trigger-recovery-bruno-mapping-save.json and trigger-recovery-bruno-mapping-readback.json.
