# BUG-20260810-initial-boot-window-absent: Startup does not create the visible boot window

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, step 2
- **Scenarios:** APP-install-first-run-provision; APP-start-installed-daemon; APP-brand-channel-visibility
- **Found:** 2026-08-10 · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`

## Summary

Lea could receive no visible resolving or provisioning surface while startup work ran because shell setup published state without creating the boot window.

## Reproduction

- **Charter:** CH-desktop-first-run-macos · **Tour:** Feature Tour
- **Environment:** macOS 26.5.1 arm64 / isolated home / wifi-fast / en-US

1. Launch the app in a fresh isolated home.
2. Observe the startup interval before the product webview is ready.
3. Inspect the shell setup path and window registry.

**Expected:** The boot window exists immediately and renders `resolving`, then later transitional states.
**Actual:** The publisher only rendered into an already-existing boot window, but startup never created one.

## Evidence

- `desktop/src-tauri/src/app_state.rs`
- `desktop/src-tauri/src/shell.rs`
- Computer Use returned `cgWindowNotFound`; visual re-verification remains platform-blocked in this run.

## Fix

- **Root cause:** `shell::setup` started asynchronous resolution without first calling the existing `boot_window::show` owner.
- **Fix commit:** `f081a1e`
- **Regression test:** `boot_window::tests::should_create_visible_boot_window_for_initial_resolution`.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`
- **Result:** The owning-layer test proves setup can create the boot window, and the installed app reaches `product`; a human macOS visual confirmation is still required because the automation could not capture a CGWindow.
