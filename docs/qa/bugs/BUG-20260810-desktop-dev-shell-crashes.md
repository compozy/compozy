# BUG-20260810-desktop-dev-shell-crashes: The development app exits before showing its first window

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, step 2
- **Scenarios:** APP-install-first-run-provision; APP-brand-channel-visibility
- **Found:** 2026-08-10 · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`

## Summary

Lea opened the development build and the process exited before any setup or product window could remain available.

## Reproduction

- **Charter:** CH-desktop-first-run-macos · **Tour:** Feature Tour
- **Environment:** macOS 26.5.1 arm64 / isolated home / wifi-fast / en-US

1. Build the unsigned development app bundle.
2. Launch it with the isolated `COMPOZY_HOME`.
3. Observe the process and app log.

**Expected:** CompozyOS remains open and presents its resolving or setup state.
**Actual:** The updater plugin rejected the missing development configuration and the process exited.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/compozyos-desktop.log`
- `desktop/src-tauri/tests/contracts.rs::should_initialize_updater_plugin_from_base_tauri_config`

## Fix

- **Root cause:** The base Tauri configuration did not initialize updater endpoints and the public key for development builds.
- **Fix commit:** `01a45c49`
- **Regression test:** `desktop/src-tauri/tests/contracts.rs::should_initialize_updater_plugin_from_base_tauri_config` failed before the fix and passes after it.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`
- **Result:** The rebuilt app stayed running and advanced through runtime resolution to the `product` state.
