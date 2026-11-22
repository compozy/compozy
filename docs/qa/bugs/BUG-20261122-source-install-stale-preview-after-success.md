# BUG-20261122-source-install-stale-preview-after-success: The Install dialog reopens with the last install's preview and gates

- **Status:** fixed, pending re-walk
- **Impact (user-side):** Friction. After a successful install, the next "Install extension from a
  local build…" opens with the old review. Install walks the old dependency gates again and then fails
  with "extension already exists"
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-distribution, source-union install
- **Scenarios:** ET-web-extension-union-install (D16 source-union dependency steps)
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md

## Summary

The source-union Install dialog keeps its preview after a successful install. Only a form edit or
closing the dialog through `onOpenChange(false)` clears it. A successful install closes the dialog
through the store phase, so neither runs. The next time the dialog opens, it shows the same path,
the same **Also installs** rows (here `acme-webdep` and `acme-off`, which were already installed and
active) and an **Install** button instead of **Review install**. Pressing Install skips the preview.
It walks the dependency gates for packages that are already installed ("Install acme-webdep?",
"Allow remote access for acme-webdep?"), then the root consent. It sends a joint request built from
the old preview, and the daemon answers 409 `extension already exists: acme-solo`. Nothing changes,
but the user confirmed three trust gates for an install that could not happen.

## Reproduction

- **Environment:** isolated lab, binary and `vite` Web from `4ce0671ff` stamped `v0.7.9`.

1. Marketplace → Add → "Install extension from a local build…", path `<lab>/fixtures/acme-solo`,
   More options → Allow an unverified package, Review install.
2. Install, then confirm the `acme-webdep` trust gate, its remote-access gate and the root consent.
   The toast reads "… installed with acme-webdep, acme-off".
3. Open Add → "Install extension from a local build…" again.

**Expected:** an empty form, or at least a fresh **Review install** that previews the current
state, so already-installed dependencies are not listed.
**Actual:** the old preview with **Also installs** `acme-webdep` and `acme-off`, and an **Install**
button. Pressing it repeats every gate and ends in 409 "extension: extension already exists:
acme-solo" inside the root consent dialog.

## Evidence

- Screenshots: `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/screenshots/07-reopen-after-success-stale-preview.png`
  and `08-stale-preview-install-error.png`, also published at
  `https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1961/4ce0671/07-reopen-after-success-stale-preview.png`
  and `…/08-stale-preview-install-error.png`.
- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/cli/stale-preview-409-request.json` and
  `stale-preview-409-response.json`.

## Pre-existing

Partly. On `origin/main`, `useExtensionInstallDialog` also leaves the preview in place after a
successful `execute`. There, the stale review offered Install and failed with the same 409. D12
(`a21255f33`) makes it worse: the stale preview now also lists **Also installs** and repeats the
dependency trust and remote-access gates before the request.

## Fix hint

Reset the preview (`resetPreview()`) after a successful install. For example, call it in `execute`
after `mutateAsync` resolves, or when the store leaves the `submitting` phase with success.

## Fix

`useExtensionInstallDialog` (`web/src/systems/marketplace/components/use-extension-install-dialog.tsx`)
now clears the preview after a successful install, and every `open()` clears it again and mounts a
fresh form (the dialog key carries a per-open session counter). A success that closes the dialog
through the store phase therefore leaves no path, review, **Also installs** rows or Install button
behind. Covered by "Should reopen on a fresh form after a successful install, without repeating its
gates" in `marketplace-components.test.tsx` (source-union install, "with dependencies").
