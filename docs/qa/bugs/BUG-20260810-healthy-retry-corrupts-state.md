# BUG-20260810-healthy-retry-corrupts-state: Retry can replace a healthy product state with an error

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-desktop-agent-headless, step 4
- **Scenarios:** APP-agent-cli-app-verbs; APP-update-recovery-state
- **Found:** 2026-08-10 · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`

## Summary

Ada sent `compozy app retry` while CompozyOS was already healthy. The shell accepted it, attempted to create a second `main` webview, and replaced the truthful product state with a load error.

## Reproduction

- **Charter:** CH-desktop-agent-headless-cli · **Tour:** Garbage Tour
- **Environment:** macOS 26.5.1 arm64 / isolated home / wifi-fast / en-US

1. Reach `state: product` in the installed desktop app.
2. Run `compozy app retry -o json`.
3. Read app status and the desktop log.

**Expected:** Retry is rejected because no failed operation exists; the app remains in `product`.
**Actual:** Retry was accepted and the shell logged that a webview labeled `main` already existed.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-003202d520fb/runtime/logs/desktop.log`

## Fix

- **Root cause:** The controller exposed the shell retry callback in every shell state, and a timed-out product webview did not release its singleton label before retry became available.
- **Fix commit:** `f081a1e`
- **Regression test:** `controller::tests::should_offer_shell_retry_only_for_a_designed_error_state`; `windowing::tests::should_wait_for_timed_out_product_window_to_release_before_retry`.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`
- **Result:** Retry returned `retry_unavailable`, and the next status remained `state: product` with the same attached runtime.
