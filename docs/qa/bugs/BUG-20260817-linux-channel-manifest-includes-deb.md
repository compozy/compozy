# BUG-20260817-linux-channel-manifest-includes-deb: Linux updater manifest exposes the manual DEB package

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-desktop-update-moment, discover app update
- **Scenarios:** REL-beta-channel-contract; APP-app-auto-update
- **Found:** 2026-08-17 · **Report:** docs/qa/reports/2026-08-17-electron-shell.md

## Summary

The beta.19 release published all immutable assets, but `latest-linux.yml` exposed both the
self-applicable AppImage and the recommendation-only DEB. The channel authority refused to advance
rather than let Electron select an unsupported package.

## Reproduction

- **Charter:** CH-electron-channel-publish-repair · **Tour:** Feature Tour
- **Environment:** exact published beta.19 Linux AppImage, DEB, and manifest

1. Prepare the Linux channel input from `latest-linux.yml`.
2. Submit it with the immutable release inventory to the channel authority.

**Expected:** The updater channel contains exactly the x64 AppImage.

**Actual:** The manifest also contained the DEB and was rejected.

## Evidence

- https://github.com/compozy/compozy/actions/runs/32011226711
- `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260816-103738-647344-lab/qa-artifacts/release-beta19-channel-failure/linux/`

## Fix

- **Root cause:** Linux channel preparation copied every builder manifest entry instead of applying
  the AppImage-only self-update contract.
- **Fix commit:** `437cea3d`
- **Regression test:** `desktop/src/release/__tests__/release.test.ts` requires the exact beta.19
  source shape to produce one AppImage with its original size and SHA-512.

## Verification

- **Retested:** exact beta.19 manifest now emits only the AppImage; hosted successor pending.
- **Result:** fixed; final channel verification is owned by the Electron shell report.
