# BUG-20261122-install-enable-500: Installing a new local extension with --enable reports an Internal Server Error and leaves it off

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-ext-secrets-binding step 1
- **Scenarios:** ET-ext-secrets-binding; ET-extension-setup-inputs
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-a-extension-inputs.md

## Summary

Bruno installs a local extension that was not installed before, with `--enable`. The package
installs, but the CLI prints `Couldn't turn on: extension_enable_failed: Internal Server Error` and
the extension stays disabled. A plain `compozy extension enable <name>` right afterwards succeeds.
This is not specific to inputs. A control fixture without inputs or `requires_env` (`acme-plain`)
fails the same way. `install --replace --enable` over an existing installation succeeds.

## Reproduction

- **Environment:** isolated `eng-qa-bootstrap` lab, binary built from `f9f47e42b` stamped `v0.7.9`,
  `[automation] enabled = false`, fresh lab home.

1. `compozy extension install <fixture-dir> --enable --allow-unverified --yes` with the required
   input supplied either through the hidden TTY prompt or through `--input api_token=…`.
2. Observe `✓ install acme-tool` and then `Couldn't turn on: extension_enable_failed: Internal Server
   Error`. Status reads `Enabled: false`, `State: disabled`.
3. `compozy extension enable acme-tool` → `Enabled in profile default.`, state `active`.

**Expected:** the extension installs and starts in one step.
**Actual:** the install-time enable fails with a masked 500. The daemon log records no error line,
so the masked cause is not observable.

## Evidence

- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/s1-install-tty.pty
- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/es1-install-input.out
- Control fixture `acme-plain` reproduced the same output; see the session debrief in the report.
- Path: `internal/daemon/extensions_install_enable.go` → `enableTargetUnlocked`. The cause is
  masked by `core.ExtensionEnableFailure(err, true)`. Whether this predates #1845 was not checked.

## Fix

## Verification
