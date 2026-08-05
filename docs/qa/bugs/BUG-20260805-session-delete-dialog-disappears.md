# BUG-20260805-session-delete-dialog-disappears: Delete confirmation disappears before deletion finishes

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Cora
- **Journey Step:** J-archive-session-without-deleting, step 7
- **Scenarios:** RT-session-list-row-actions; ET-web-sessions-catalog-modal
- **Found:** 2026-08-05 · **Report:** docs/qa/reports/2026-08-05-session-archive-coderabbit.md

## Summary

When Cora confirms deletion from the global session catalog, the confirmation and its parent catalog
disappear before the request finishes. A slow request therefore leaves no progress state or reliable
signal that deletion is still running.

## Reproduction

- **Charter:** CH-archive-session-catalog · **Tour:** Feature Tour
- **Environment:** laptop / wifi-fast with an intentionally delayed DELETE response / en-US

1. Open the global Sessions catalog.
2. Open a stopped session row's three-dot menu and choose **Delete**.
3. Delay the session DELETE response and confirm **Delete session**.
4. Observe the interface while the request remains pending.

**Expected:** The confirmation stays visible, shows **Deleting**, blocks dismissal, and keeps the
Sessions catalog mounted until the request settles.
**Actual:** Both the confirmation and Sessions catalog disappear immediately while the DELETE
request remains pending.

## Evidence

- `docs/qa/evidence/2026-08-05-session-archive-coderabbit/CH-archive-session-catalog-delete-pending-closed.png`
- A direct runtime read returned HTTP 200 for `sess-50566a506db0b289` while the browser's DELETE
  request remained intentionally pending, confirming the session had not been removed.

## Fix

- **Root cause:** The session lifecycle hook cleared its delete target before starting the mutation,
  and the global catalog explicitly dismissed itself when Delete was selected. Escape from the
  pending confirmation could then dismiss the underlying catalog.
- **Fix commit:** PR #309 CodeRabbit remediation commit containing this record
- **Regression test:** `web/src/systems/session/hooks/__tests__/use-session-actions.test.tsx` and
  `web/src/systems/os/components/__tests__/sessions-modal.test.tsx`

## Verification

- **Retested:** 2026-08-05, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-05-session-archive-coderabbit.md`
- **Result:** With DELETE held open, the confirmation and catalog remained mounted, **Deleting**
  stayed visible, Cancel stayed disabled, and Escape changed neither dialog. With the delay removed,
  deletion completed, the confirmation closed, the catalog remained open, and a direct runtime read
  returned HTTP 404.
