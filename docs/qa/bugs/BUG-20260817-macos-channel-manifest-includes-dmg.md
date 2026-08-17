# BUG-20260817-macos-channel-manifest-includes-dmg: macOS updater manifest exposes DMGs alongside ZIPs

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-desktop-update-moment, discover app update
- **Scenarios:** REL-beta-channel-contract; APP-app-auto-update
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

The beta.18 release published all 30 assets, but the merged macOS channel manifest listed both DMG
installers and ZIP updater archives. The channel authority correctly refused the ambiguous
generation instead of exposing unsupported self-apply payloads.

## Reproduction

- **Charter:** CH-electron-channel-publish-repair · **Tour:** Feature Tour
- **Environment:** exact beta.18 arm64 and x64 release manifests

1. Merge the two architecture-specific `latest-mac.yml` files.
2. Submit the merged manifest and immutable release inventory to the channel authority.

**Expected:** The channel contains exactly one arm64 ZIP and one x64 ZIP.

**Actual:** The merged manifest also contained both DMGs and was rejected.

## Evidence

- https://github.com/compozy/compozy/actions/runs/32004091015
- `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260816-103738-647344-lab/qa-artifacts/release-beta18-channel-failure/`

## Fix

- **Root cause:** the merge preserved every source-manifest file instead of selecting the updater's
  ZIP transport contract.
- **Fix commit:** `361c785e`
- **Regression test:** `desktop/src/release/__tests__/release.test.ts` feeds real DMG+ZIP source
  shapes and requires a strict two-ZIP merged result.

## Verification

- **Retested:** exact beta.18 manifests now emit only arm64/x64 ZIPs; hosted successor pending.
- **Result:** fixed; final channel verification is owned by the Electron shell report.
