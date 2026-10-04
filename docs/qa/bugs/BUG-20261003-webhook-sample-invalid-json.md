# BUG-20261003-webhook-sample-invalid-json: The webhook preview shows invalid sample JSON

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-24, read the incoming webhook sample before saving
- **Scenarios:** ET-web-jobs-triggers-catalog
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The live Trigger preview presents a JSON sample that a user cannot parse or reuse. Quotes inside
the webhook payload string are printed literally, making the containing envelope invalid.

## Reproduction

- **Charter:** CH-trigger-detail-rule-page · **Tour:** Feature Tour
- **Environment:** Chrome, desktop 1512x862, wifi-fast, en-US

1. Open the incoming webhook Trigger editor from Triggers.
2. Select Show live preview and read Sample event.
3. Copy the displayed envelope into a JSON reader.

**Expected:** The displayed envelope is valid JSON and preserves the payload as a string.
**Actual:** Its payload line contains unescaped nested quotes. JSON parsing fails at line 7,
column 19. The separate saved-definition Inspect sample already escapes its payload correctly.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: trigger-preview-error-bruno-corrected-result.json
contains the original rendered sample; trigger-preview-sample-json-diagnostic.json captures
the later engineering read and parser failure. Its curl block is shell text and is not counted
as a JSON failure. trigger-rule-webhook-inspect.png shows the independent correctly escaped sample.

## Fix

- **Root cause:** SampleEventCard surrounds raw strings with quote characters instead of
  serializing labels, keys and values as JSON strings.
- **Fix commit:** self (the commit that records this verified repair)
- **Regression test:** Existing automation-trigger-form.test.tsx owns the rendered preview.
  Parse the actual rendered envelope and its nested payload to prove valid, preserved JSON.

## Verification

- **Retested:** 2026-10-03, Bruno, CH-trigger-detail-rule-page.
- **Result:** Fresh Bruno preview parses the rendered envelope and its nested payload without error. Before/after owning tests and trigger-recovery-bruno-mapping-save.json / trigger-recovery-sample-json.png prove the repair.
