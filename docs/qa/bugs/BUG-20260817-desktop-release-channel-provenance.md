# BUG-20260817-desktop-release-channel-provenance: Packaged desktop reports a development channel and incomplete runtime provenance

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, first launch
- **Scenarios:** APP-install-first-run-provision; APP-brand-channel-visibility
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

All three beta.17 packaged smokes reached the shipped app but could not prove a beta installation:
the Electron main process compiled the fallback `development` channel and the bundled runtime wrote
a provenance marker without the app version, channel, or runtime version.

## Reproduction

- **Charter:** CH-electron-offline-first-run-macos / CH-electron-offline-first-run-linux · **Tour:** Feature Tour
- **Environment:** signed beta.17 DMG and AppImage, empty isolated homes

1. Launch the signed package with no operator runtime on `PATH`.
2. Wait for bundled provisioning.
3. Read `app.json` and `bin/.desktop-provenance.json`.

**Expected:** Both records identify beta.17 and its lockstep bundled runtime on channel `beta`.

**Actual:** `app.json` reported `development`; the provenance marker could not establish the
package/runtime identity required by the release smoke.

## Evidence

- https://github.com/compozy/compozy/actions/runs/31988469820

## Fix

- **Root cause:** `build-main.ts` did not compile the release plan channel into Electron, and the
  bootstrap marker carried only owner and digest.
- **Fix commit:** `94e2ce7`
- **Regression test:** canonical desktop release, CLI bootstrap, update apply, and restore suites
  require the full app/channel/runtime marker and preserve it across update recovery.

## Verification

- **Retested:** focused suites and the hosted beta.20 Linux and macOS packaged smokes passed before the release was removed by explicit decision.
- **Result:** verified for packaged provenance; the retained run receipt is https://github.com/compozy/compozy/actions/runs/32017263255.
