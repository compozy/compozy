# BUG-20260810-app-control-timeout: Structured app commands time out while the desktop is running

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-desktop-agent-headless, step 3
- **Scenarios:** APP-agent-cli-app-verbs
- **Found:** 2026-08-10 · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`

## Summary

Ada could read app status, but `diagnose`, `update`, and `retry` returned `app_control_unavailable` while the desktop process and socket were live.

## Reproduction

- **Charter:** CH-desktop-agent-headless-cli · **Tour:** Feature Tour
- **Environment:** macOS 26.5.1 arm64 / isolated home / wifi-fast / en-US

1. Start the installed desktop app.
2. Run `compozy app diagnose -o json` or `compozy app update --check -o json`.
3. Compare the CLI result with a raw newline-framed socket request.

**Expected:** The shell responds to one versioned newline-framed request without requiring the client to close its write side.
**Actual:** The Rust server waited for EOF while the Go client waited for the response, so both sides stalled.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt`
- `desktop/src-tauri/src/control.rs::tests::should_round_trip_every_versioned_control_primitive`

## Fix

- **Root cause:** The server read the connection to EOF instead of reading one bounded newline-delimited control frame.
- **Fix commit:** `0805f649`
- **Regression test:** `desktop/src-tauri/src/control.rs::tests::should_round_trip_every_versioned_control_primitive` now keeps the write side open and passes.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`
- **Result:** `diagnose`, `update --check`, and `open /sessions/qa-desktop-link` returned structured successful results from the installed app.
