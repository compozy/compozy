# BUG-20260817-signed-macos-x64-digest-drift: macOS x64 signing changes the bundled runtime digest

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-desktop-first-run, bundled runtime verification
- **Scenarios:** APP-install-first-run-provision; REL-beta-installer-provenance
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

The beta.17 retry provisioned on Linux and macOS arm64, but macOS x64 rejected its own bundled Go
runtime because the digest recorded before code signing no longer matched the signed Mach-O bytes.

## Reproduction

- **Charter:** CH-electron-offline-first-run-macos · **Tour:** Feature Tour
- **Environment:** signed beta.17 x64 DMG, empty isolated home

1. Build the x64 runtime and record its unsigned digest.
2. Let the release builder sign the packaged runtime.
3. Launch the signed package and verify the bundled runtime against the recorded digest.

**Expected:** Signing preserves a verifiable identity for the same runtime payload.

**Actual:** `codesign` added `LC_CODE_SIGNATURE` and expanded `__LINKEDIT`, so the byte-for-byte
unsigned hash rejected the valid signed runtime.

## Evidence

- https://github.com/compozy/compozy/actions/runs/31992278359
- `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260816-103738-647344-lab/qa-artifacts/release-beta17-retry/`

## Fix

- **Root cause:** provenance hashing treated a signed Mach-O as an unchanged unsigned file.
- **Fix commit:** `1a0b52d`
- **Regression test:** the canonical desktop release suite reconstructs the unsigned Mach-O header
  and proves its normalized digest equals the exact failed signed package.

## Verification

- **Retested:** the exact beta.17 x64 artifact, focused desktop suites, and the hosted beta.20 macOS x64 packaged smoke passed before the release was removed by explicit decision.
- **Result:** verified for the signed runtime digest; the retained run receipt is https://github.com/compozy/compozy/actions/runs/32017263255.
