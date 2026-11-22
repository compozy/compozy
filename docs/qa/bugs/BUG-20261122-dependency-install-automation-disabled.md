# BUG-20261122-dependency-install-automation-disabled: Dependency installs fail on a daemon with automation disabled

- **Status:** open (the trigger predates #1845; #1845 makes it block every dependency install)
- **Impact (user-side):** Blocker for `--with-dependencies` and the Web Add flow when `[automation] enabled = false`
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-extension-dependencies step 3
- **Scenarios:** ET-extension-dependencies; ET-web-extension-union-install (dependency part)
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies.md
- **Related:** BUG-20261122-install-enable-500 (compozy/compozy#1959). This walk found its trigger.

## Summary

Bruno runs a daemon with `[automation] enabled = false` and installs an extension with
`--with-dependencies`. The first dependency installs but stays disabled, and the install stops:

```text
{"error":"dependency \"acme-core\": daemon: dependency installed but not enabled: Internal Server Error (dependencies installed: acme-core)","code":"extension_install_failed","dependency":"acme-core","dependencies_installed":["acme-core"]}
```

The same command run again fails earlier with an unmasked cause:

```text
{"error":"dependency \"acme-core\": daemon: extension automation previewer is required","code":"extension_install_failed","dependency":"acme-core"}
```

Each dependency install calls `installPrepared` with `Enable: true`
(`internal/daemon/extension_dependencies_install.go:117-129`). That is the install-time enable path
of #1959. A dependency that fails to enable aborts the whole dependency install, so with automation
disabled no dependent can be installed with its dependencies.

## Reproduction

- **Environment:** isolated `eng-qa-bootstrap` lab, binary built from `08bd19a60` and stamped
  `v0.7.9`, fresh home, `[automation] enabled = false`.

1. `compozy extension install <acme-tool> --allow-unverified --with-dependencies
   --allow-unverified-dependency acme-core --allow-unverified-dependency acme-base --yes -o json`.
   `acme-tool` requires the local `acme-core` and `acme-base` packages.
2. Observe the first error above, exit 69. `acme-core` is installed and `disabled`.
3. Run the same command again and observe the `automation previewer is required` error.
4. Control 1: `compozy extension install <acme-leaf> --allow-unverified --enable --yes -o json` on
   the same daemon returns `enable_error: {"error":"Internal Server Error"}`. This is #1959.
5. Control 2: a plain `compozy extension enable acme-core` succeeds on the same daemon.
6. Set `[automation] enabled = true` and restart the daemon. Steps 1 and 4 then succeed:
   `install --enable` reports `enabled: true` and `state: active`, and the dependency install
   reaches the next gate (the gateway confirmation for `acme-base`).

**Expected:** the dependency is installed and enabled, whatever the automation setting.
**Actual:** with automation disabled, the install-time enable fails with a masked 500. A retry then
reports `daemon: extension automation previewer is required`.

## Evidence

- `docs/qa/evidence/2026-11-22-gh-1845-pr-b/cli/transcript.md`: entries "S3 unverified deps
  consented, no gateway", "S3 rerun after partial", the two "probe: plain install acme-leaf --enable"
  entries, and "probe enable acme-core (automation disabled)".
- `docs/qa/evidence/2026-11-22-gh-1845-pr-b/cli/daemon-automation-disabled.log`: daemon log of the
  automation-disabled run.
- The error comes from `previewExtensionAutomation` (`internal/daemon/extensions_inventory.go:364-365`).
  It returns the error when `s.automation == nil`, which happens when automation is disabled.
  `origin/main` (`9d5e297d7`) has the same check, so the trigger predates #1845. #1845 adds the
  dependency path that turns it into a blocker.

## Fix

**Disposition: not fixed here (QA walk; product code untouched).**

The lowest-risk fix is in the shared function, so every caller benefits. With no automation
previewer, `previewExtensionAutomation` should report no automation changes instead of an error,
because a disabled automation subsystem starts nothing. Then re-walk ET-extension-dependencies
step 3 and #1959 on a daemon with `[automation] enabled = false`.

## Verification

Pending upstream fix. The rest of this walk ran with `[automation] enabled = true`.
