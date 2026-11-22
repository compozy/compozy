# BUG-20261122-install-enable-500: Installing a new local extension with --enable reports an Internal Server Error and leaves it off

- **Status:** open-upstream (pre-existing, not caused by #1845)
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

**Disposition: pre-existing, not fixed here.**

**Evidence that #1845 did not introduce it.**

- `git diff origin/main...HEAD` is empty for every file on the install → enable path:
  `internal/daemon/extensions_install_enable.go`, `extensions_enable.go`, `extensions_lifecycle.go`
  (`Install`, `commitPreparedInstall`), `extensions_inventory.go` (`previewExtensionTarget`),
  `extensions_scope.go` (`installedTargetStatus`), `extensions_events.go`, and `extensions.go`
  (`reloadExtensions`, `syncExtensionConsumers`).
- #1845 reaches this path only through the input reader in `populateExtensionInputStatus` and the
  launch env resolution. The control fixture `acme-plain` declares no inputs and no `requires_env`,
  so both are no-ops for it, and it fails the same way.

**Where it fails (lab daemon.log).**

- During the `acme-plain` install, the enable reload loads the extension active (11:25:40.656
  `extension.lifecycle.loaded acme-plain active=true`). A second global reload at 11:25:42.3 then
  shuts it down and reloads without it.
- That is `rollbackGlobalExtensionLifecycle` running after a step that follows a successful reload
  in `enableTargetUnlocked` (`internal/daemon/extensions_enable.go`): the consumer sync inside
  `reloadExtensions` (`extensions.go:104`), `installedTargetStatus` (`:58`), or
  `recordExtensionEnableEvents` (`:64`).
- Install runs `enableTargetUnlocked` with `prepared.target` under the install lock
  (`extensions_install_enable.go:23-26`). A later plain `enable` runs it under `withName` with the
  default target and succeeds.

**Why no log line exists.**

- `enableInstalledExtension` stores the failure only as `item.InstallEnableError`
  (`extensions_install_enable.go:53`).
- `installEnableError` masks it through `core.ExtensionEnableFailure(err, true)` (`:67`) and never
  logs the unmasked error.
- The static read did not single out which of the three steps fails. An upstream fix should log the
  unmasked cause at warn level in `enableInstalledExtension`, then reproduce with a fresh global
  `install --enable` of a no-input fixture.

## Verification
