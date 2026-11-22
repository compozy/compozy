# BUG-20261122-local-dependency-elsewhere-retry-no-unblock: The local `--replace` retry for an `installed_elsewhere` dependency never unblocks the root

- **Status:** verified
- **Impact (user-side):** Blocked. The user runs the offered command, it succeeds, and the root
  still cannot be installed with any flag
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, install with dependencies
- **Scenarios:** ET-extension-dependencies step 12
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md

## Summary

A `local_path` dependency installed only for profile `other`, whose source now has different bytes,
is previewed as `installed_elsewhere` with
`retry_command: compozy extension install <dep dir> --replace --scope global --profile other`.
Running that command (with `--allow-unverified --yes`) succeeds, and the `other` copy then has the
source's checksum. The preview of the root then reports the dependency as `missing` with
`consent_required: true`. But installing the root with `--with-dependencies
--allow-unverified-dependency <dep>` refuses with `extension_exists` and `dependency: "<dep>"`.
The documented behavior is that a dependency installed for another profile with the same bytes is only
attached, so the dependency should be attached here.

A direct local install of the dependency into `default` is refused as well: without `--replace` it
returns `extension_exists`, and with `--replace` it returns `extension not found`. No command
attaches a `local_path` package that another profile already has, so `install --replace` cannot be
the command that unblocks the root. The marketplace variant (`compozy extension update <dep> --scope
global --profile other`) works: after it runs, the root installs and the dependency is attached.

## Reproduction

- **Environment:** isolated lab, binary from `43f54e4c6` stamped `v0.7.9`.

1. `compozy profile create other`, then `compozy extension install <acme-p12dep> --allow-unverified
   --enable --yes --profile other`, then `compozy profile use default`.
2. Edit `acme-p12dep/extension.toml`. The description is enough; keep the version.
3. `POST /api/extensions/preview-install` for `acme-p12`, which requires `acme-p12dep` as a
   `local_path` dependency: it returns `installed_elsewhere` with the `install … --replace --scope
   global --profile other` retry command.
4. Run that command with `--allow-unverified --yes`: exit 0.
5. `compozy extension install <acme-p12> --allow-unverified --with-dependencies
   --allow-unverified-dependency acme-p12dep --enable --yes`.

**Expected:** `acme-p12dep` is attached to `default` and `acme-p12` installs. Otherwise, the
daemon offers no retry command for a `local_path` package installed elsewhere and gives guidance
that works.
**Actual:** exit 65 with `extension_exists`, `dependency: "acme-p12dep"`. Nothing is installed in
`default`.

## Evidence

- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-s9-s12/cli/transcript.md`: the entries "S12a run retry
  command", "S12a install acme-p12 after retry", "S12a preview acme-p12 after retry", "S12a install
  acme-p12 after retry human" and the two "S12a diag …" entries.

## Pre-existing

No. The `install <source> [--replace]` retry command and the claim that a same-bytes dependency is
attached come from PR B (`43f54e4c6`, `internal/daemon/extension_dependencies_retry.go`). The
commit path refuses because a `local_path` candidate has no `published` record. In
`installPrepared` (`internal/daemon/extensions_install.go`), only published candidates reach the
reinstall/attach branch. A `local_path` candidate goes to the fresh-install commit, and that commit
refuses because the name is already registered.

## Fix

The QA suspicion was close but not exact: a `local_path` candidate does reach the reinstall branch
(`published` is set for local bundles too). It was refused there for two reasons:

- `PreparedLocalManagedInstall.ValidateReinstall` returns `extension_exists` when the request has no
  `replace` (`internal/extension/install_local_preparation.go`), and a dependency install never sets
  it. `validateReinstallAuthority` (`internal/daemon/extensions_reinstall.go`) now skips the
  replacement guards when a dependency install (`keepInstalled`) matches the installed bytes. Nothing
  is replaced, so no replace authority is needed (D9).
- Local candidates never set `enrollAttachment`, so even a passing reinstall would not attach the
  dependent's cell. `installDependencyNode` (`internal/daemon/extension_dependencies_install.go`)
  now enrolls the attachment for every dependency source, so the same bytes are attached as they are
  for a marketplace dependency.

The offered local command now carries `--allow-unverified`: `compozy extension install <dir> --replace
--allow-unverified --scope global --profile other`. A side-load is always unverified and the CLI never
offers that consent on its own; in a terminal it still asks before installing, so `--yes` is
deliberately left out. The checksum-unverified diagnostic now reads "The extension source did not
provide a verifiable checksum." and no longer mentions a marketplace for a local folder.

Regression test: `TestExtensionDependencyRetryCommand/Should name a local reinstall that unblocks the
root by attaching the same bytes` (`internal/daemon/extension_dependencies_test.go`). It asserts the
new command, runs the offered reinstall, and then requires the root install to succeed with
`dep-core` attached to both profiles.

The direct `install <dir>` into a profile without the package still refuses (`extension_exists`,
or `extension not found` with `--replace`). That path is not a retry command, so this fix leaves it
unchanged.

## Verification

Re-walked on 2026-11-22 against head `f0b712302` (binary stamped `v0.7.9`, fresh isolated lab,
`[automation] enabled = false`). The report is the "Third re-walk" section of
`docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md`, and the evidence is in
`docs/qa/evidence/2026-11-22-gh-1845-pr-b-s12-local/`.

1. `acme-p12dep` 0.1.0 installed for `other` only, then its source description changed. The preview of
   `acme-p12` gives `installed_elsewhere` with `retry_command: compozy extension install
   <lab>/fixtures/acme-p12dep --replace --allow-unverified --scope global --profile other`. The
   `--with-dependencies` install exits 2 with `extension_dependencies_required`, the same top-level
   `retry_command`, and human `Run first: …`.
2. That command, typed exactly as shown in a PTY (`s12-local-retry-tty.pty`), asks "This extension
   checksum is unverified. Continue with allow_unverified? [y/N]". After `y` it exits 0. The `other`
   copy now has the source checksum `084282be…`.
3. The preview then reports `acme-p12dep` as `missing` with digest `084282be…`. `install acme-p12
   --allow-unverified --with-dependencies --allow-unverified-dependency acme-p12dep --enable --yes`
   exits 0 with `dependencies_installed: ["acme-p12dep"]`, and `acme-p12` is `enabled: true, active`.
4. `extension list --profile default` and `--profile other` both show `acme-p12dep` 0.1.0 `active`
   with checksum `084282be…`. `other` keeps its own installation (`installation_profile: other`).

The marketplace variant still unblocks the root, and step 9 and the G-9 workspace hint show no
regression.
