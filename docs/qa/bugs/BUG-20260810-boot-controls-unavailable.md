# BUG-20260810-boot-controls-unavailable: Startup recovery controls do nothing

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, step 2
- **Scenarios:** APP-install-first-run-provision
- **Found:** 2026-08-10 · **Report:** docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md

## Summary

When CompozyOS could not start its runtime, Lea could neither retry nor open diagnostics. Both controls reported that app control was unavailable, leaving no recovery path inside the app.

## Reproduction

- **Charter:** CH-desktop-first-run-macos · **Tour:** Feature Tour
- **Environment:** macOS development app / laptop / wifi-fast / en-US / isolated Compozy home

1. Start the current desktop app with an installed but stopped isolated runtime.
2. Wait for `CompozyOS could not start`.
3. Select `Retry operation`, then `Show diagnostics`.

**Expected:** Retry invokes the registered shell retry, and diagnostics shows the app and runtime log paths.
**Actual:** Both controls showed `The app control is unavailable.` The equivalent `compozy app retry` and `compozy app diagnose` commands succeeded.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/boot-retry-blocked.jpeg`
- Independent CLI read path: `compozy app retry -o json` returned `accepted: true`; `compozy app diagnose -o json` returned both log paths.

## Fix

- **Root cause:** The initial boot webview is created from `tauri.conf.json`, while the minimal `__TAURI__.core.invoke` bridge was installed only on boot windows recreated programmatically. The initial page therefore had no callable bridge even though the Rust command was registered.
- **Fix commit:** working tree (the parent remediation has not been committed)
- **Regression test:** scripted macOS replay in this report; an automated Tauri initial-window replay is tracked in `docs/qa/automation-backlog/desktop-boot-control-bridge.md` because the existing Rust mock does not execute configured webview JavaScript.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md
- **Result:** `Show diagnostics` rendered both isolated log paths. `Retry operation` moved to `Starting CompozyOS`, then returned to the designed error state with the current labels and controls; no unavailable-control message or stale busy state remained.
