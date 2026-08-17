# BUG-20260817-macos-finalized-dmg-integrity-stale: macOS channel input keeps the pre-notarization DMG digest

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-publish-compozy-beta, channel publication
- **Scenarios:** REL-beta-channel-contract; REL-beta-installer-provenance
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

The GitHub beta.17 release became public with complete immutable assets, but the desktop channel
authority refused it because `latest-mac.yml` still described the DMG before notarization and
stapling changed the finalized file.

## Reproduction

- **Charter:** CH-electron-channel-publish-repair · **Tour:** Feature Tour
- **Environment:** published beta.17 release and channel authority

1. Build the macOS DMG and its initial updater manifest.
2. Notarize and staple the DMG.
3. Ask the channel authority to verify the finalized asset against the original manifest.

**Expected:** The manifest carries the finalized immutable DMG size and SHA-512.

**Actual:** The post-staple bytes no longer matched the manifest, so publication stopped safely.

## Evidence

- https://github.com/compozy/compozy/actions/runs/31997088968

## Fix

- **Root cause:** finalization validated the DMG but did not refresh its manifest integrity fields
  after notarization and stapling.
- **Fix commit:** `bd5ecef`
- **Regression test:** the canonical desktop release suite requires finalization to rewrite size and
  SHA-512 from the finalized DMG before the authority reads it.

## Verification

- **Retested:** exact finalized DMG integrity passed locally; hosted successor pending.
- **Result:** fixed; final channel verification is owned by the Electron shell report.
