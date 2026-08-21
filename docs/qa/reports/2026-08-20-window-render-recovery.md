# QA Run Report — 2026-08-20 — window-render-recovery

- **Scope:** Desktop window rendering after workspace-scoped window-manager settings hydration.
- **Cadence tier:** targeted
- **Build:** `01313427` + working tree · **Environment:** local development daemon at `http://127.0.0.1:2123` and Web at `http://localhost:3000`
- **Started:** 2026-08-20T22:17:19-03:00 · **Status:** pass

## Persona

Bruno used the desktop shell in Chrome with a fast local connection and `en-US` locale.

## Result

| Charter | Journey / Scenario | Tour | Status |
|---|---|---|---|
| CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-desktop-shell-lifecycle | Feature Tour | Pass |

## Session debrief

- The scoped settings response encoded `diagnostics` and `extension_defaults` as `[]`.
- `global_shortcuts` preserved the configured `palette.summon.global` intent while omitting registration status until the desktop shell reports it.
- Opening Agents from the dock rendered an interactive Agents window. The `Sync paused · retrying` warning did not appear.
- A full reload at `/agents` kept the window rendered and the warning absent.
- Evidence: `../evidence/2026-08-20-window-render-recovery/agents-window-after-reload.png`.

## Runtime errors observed

No Compozy-owned browser error, failed window-manager request, or sync warning occurred in the verdict window.

## Final status

**Verdict: PASS.** The real dock-to-window path and reload recovery both completed successfully.
