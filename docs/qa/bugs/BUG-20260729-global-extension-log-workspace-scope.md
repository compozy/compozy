# BUG-20260729-global-extension-log-workspace-scope: Global extension logs inherit the active workspace

- **Status:** verified
- **Impact (user-side):** Breaks global extension log management in Web
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-dev-lifecycle, inspect retained and live logs
- **Scenarios:** ET-web-extension-logs-panel
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-ext-improvs.md
- **Origin:** Task 11 isolated browser replay

## Summary

The installed-extension detail resolved a global published extension correctly, but its logs hook
used the currently selected workspace rather than the resolved extension instance identity. The Web
panel therefore requested a workspace dev overlay that did not exist and rendered `extension is not
dev linked`; the equivalent global HTTP read returned `200` with an empty log collection.

## Reproduction

1. Select a workspace that has no dev link for an installed global extension.
2. Open the extension detail through Marketplace and inspect the Logs panel.
3. Compare the panel request with `GET /api/extensions/{name}/logs` without a workspace query.

**Expected:** A global fallback uses the global log instance; a dev overlay uses its returned
`workspace_id`.

**Actual:** The global fallback inherited the active workspace and failed as a missing dev link.

## Evidence

- Browser replay: `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/browser/extension-management.json`.
- The global HTTP read returned `200 {"logs":[]}` while the Web panel reported the nonexistent
  workspace overlay.

## Fix

- **Root cause:** `useExtensionDetailState` forwarded the inventory selection scope to
  `useExtensionLogs`. Inventory scope means “prefer this workspace overlay, otherwise return the
  global row”; it is not the identity of the row the daemon actually returned.
- **Correction:** The logs query and SSE stream now use `extension.workspace_id`, with an absent
  value selecting the global instance.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical detail-state hook suite proves that an active workspace leaves
  global logs unscoped and that a returned dev overlay retains its exact workspace identity.

## Verification

- The regression failed before the production change with `workspaceId: "ws_active"` for a global
  extension and passes after the correction.
- The same live browser tab updated through HMR and rendered the global empty state without the
  missing-dev-link error.

