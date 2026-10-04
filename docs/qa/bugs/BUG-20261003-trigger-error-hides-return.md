# BUG-20261003-trigger-error-hides-return: A failed Trigger detail has no catalog return action

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-24, recover from an unavailable Trigger link
- **Scenarios:** ET-web-trigger-detail-rule-page
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A missing Trigger link displays the server's useful error, but removes the breadcrumb and offers
no way back to Triggers. The Dock restores the same unavailable detail rather than its catalog.

## Reproduction

- **Charter:** CH-trigger-detail-rule-page · **Tour:** Feature Tour
- **Environment:** Chrome, desktop 1512x862, wifi-fast, en-US

1. Open a Trigger from the catalog and retain its detail URL.
2. Visit that URL with the identifier replaced by trg-missing-editorial-publication.
3. Read the not-found error and look for a catalog return action.
4. Hide and restore Triggers through its Dock item.

**Expected:** The unavailable detail keeps an explicit route back to the Trigger catalog.
**Actual:** Only Unable to load details and the not-found message are shown. No Back to Triggers
or parent breadcrumb exists; restoring the app returns to the same error.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: trigger-rule-bruno-malformed-ready.json,
trigger-rule-missing-ready.png, trigger-rule-bruno-managed-entry-observe.json,
trigger-rule-bruno-malformed-dock-recovery.json and trigger-preview-error-bruno-ended.json.
The transient native-history URL observation is retained without asserting its root cause.

## Fix

- **Root cause:** TriggerDetailPanel supplies its existing Back to Triggers action only when the
  record is absent without an error. A real missing-id response takes the preceding error branch,
  which omits that action; loaded-only topbar navigation is also absent.
- **Fix commit:** 78133b4f0f2c477f7fbc48b9abe1678463c0c5d0
- **Regression test:** The existing trigger-detail-panel.test.tsx suite owns error presentation
  and the onBack action. Extend its error case to prove that users can invoke catalog navigation.

## Verification

- **Retested:** 2026-10-03, Bruno, CH-trigger-detail-rule-page.
- **Result:** Fresh Bruno malformed-url entry exposes Back to Triggers; activating it reaches the searchable catalog and the saved webhook detail. Evidence: trigger-recovery-bruno-catalog-return-ready.json and trigger-recovery-back-action.png.
