# BUG-20261122-local-dependency-elsewhere-retry-no-unblock: The local `--replace` retry for an `installed_elsewhere` dependency never unblocks the root

- **Status:** open
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

## Verification

Pending fix.
