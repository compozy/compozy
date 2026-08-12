# BUG-20260812-workspace-extension-detail-missing: Workspace extension detail disappears from Marketplace

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-dev-lifecycle, step 4
- **Scenarios:** ET-web-extension-logs-panel
- **Found:** 2026-08-12 · **Report:** docs/qa/reports/2026-08-11-frontend-performance.md

## Summary

Bruno could see a dev-linked extension in the workspace Marketplace list, but opening that same
extension reported that it was no longer in Marketplace. The missing detail made the Web logs panel
unreachable even though the extension was active and its logs were available through the API.

## Reproduction

- **Charter:** CH-extension-dev-recovery · **Tour:** Interrupt Tour
- **Environment:** Desktop Chromium, isolated local daemon and Web app, en-US

1. Link `epoch-probe` to the active workspace with `compozy extension dev`.
2. Confirm the workspace Marketplace list shows `epoch-probe` as installed.
3. Open its workspace-scoped Marketplace detail route.

**Expected:** The installed detail opens and its Logs region shows the workspace instance history.
**Actual:** The detail returned 404 and rendered “This item is no longer in marketplace.”

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/web-extension-detail-gap.png`
- `/Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/web-extension-logs-pass.png`
- `/Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/api-extension-logs.json`

## Fix

- **Root cause:** Marketplace list and installed-detail projection called the global extension list even when the request carried workspace scope. Both now use the existing scoped extension read service with the transport-resolved actor, and reject actor/workspace mismatches.
- **Fix commit:** `72170640`
- **Regression test:** `internal/api/core/marketplace_test.go` — `Should resolve workspace extension listings and details from one scoped projection`; `internal/daemon/native_marketplace_tools_test.go` — `Should preserve the trusted workspace actor for scoped extension discovery`

## Verification

- **Retested:** 2026-08-12, same persona/journey · **Report:** docs/qa/reports/2026-08-11-frontend-performance.md
- **Result:** The deep link opened after a fresh daemon/Web restart, rendered `epoch-two-observed`, retained the row while Follow was paused, restored it after page reload, and matched an independent workspace-scoped HTTP snapshot. Browser errors were empty and teardown reported `clean: true`.
