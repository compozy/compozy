# BUG-20260817-desktop-smoke-local-isolation: Desktop package smoke collides with local macOS runtime paths

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, release package acceptance
- **Scenarios:** APP-install-first-run-provision; REL-beta-installer-provenance
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

The official packaged-desktop smoke worked on clean hosted runners but failed in a concurrent local
macOS QA session: its generated app socket exceeded the platform limit, its daemon used occupied
port `2123`, and its health comparison treated `/var` and canonical `/private/var` as different homes.

## Reproduction

- **Charter:** CH-electron-offline-first-run-macos · **Tour:** Feature Tour
- **Environment:** signed beta.18 arm64 DMG, local macOS with an unrelated daemon on port 2123

1. Run `scripts/smoke-desktop-release-artifact.sh` with the default macOS `TMPDIR`.
2. Observe `app.sock` fail with `EINVAL`.
3. Shorten the path and observe the isolated daemon collide with port 2123.
4. Allocate a unique port and observe the app reach `product` while the smoke times out comparing
   `/var/...` with the daemon's canonical `/private/var/...` home.

**Expected:** Every invocation owns a short home, UDS path, and free HTTP port, and accepts the
daemon's canonical home identity.

**Actual:** Local release QA could not reach a valid verdict even though the signed package was healthy.

## Evidence

- Red path failure: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-desktop-smoke.OQl7pr/`
- Red port failure: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyds.zPCUkr/`
- Red alias comparison: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyds.55tryY/`
- Green retest: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyds.3iekJu/`

## Fix

- **Root cause:** the smoke assumed hosted-runner path length, port availability, and path spelling
  instead of owning all three pieces of its isolated runtime envelope.
- **Fix commit:** `560cd17`
- **Regression test:** the exact signed beta.18 DMG smoke is the owning integration suite; it failed
  at each exposed boundary before the fix and passed end to end afterward.

## Verification

- **Retested:** 2026-08-17, same signed arm64 DMG and concurrent local daemon.
- **Result:** `desktop release smoke: PASS`, followed by `TEARDOWN_CLEAN=true` with both registered
  app and daemon processes stopped.
