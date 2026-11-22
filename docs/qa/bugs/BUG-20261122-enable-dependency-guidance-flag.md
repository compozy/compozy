# BUG-20261122-enable-dependency-guidance-flag: The enable refusal suggests a flag that enable does not have

- **Status:** fixed, pending re-walk
- **Impact (user-side):** Confusion; the suggested command fails
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-extension-dependencies step 7
- **Scenarios:** ET-extension-dependencies
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies.md

## Summary

`compozy extension enable acme-tool` while its dependency `acme-core` is disabled correctly exits 2
with `extension_dependencies_required` and lists `acme-core (disabled)`. The human message then
says `Pass --with-dependencies to install them first (experimental).` But `enable` has no
`--with-dependencies` flag (`Error: unknown flag: --with-dependencies`), and a disabled dependency
needs `compozy extension enable acme-core`, not an install.

## Reproduction

1. Install `acme-tool` with its dependencies `acme-core` and `acme-base`.
2. `compozy extension disable acme-core --force`, then `compozy extension disable acme-tool`.
3. `compozy extension enable acme-tool`:
   `error: extension dependencies unmet: acme-core (disabled). Pass --with-dependencies to install them first (experimental). (extension_dependencies_required)`, exit 2.
4. `compozy extension enable acme-tool --with-dependencies`: `Error: unknown flag: --with-dependencies`.

**Expected:** guidance that matches the command and the state, for example
`Enable it first: compozy extension enable acme-core`. For a `missing` dependency, the guidance
should point to `compozy extension install <dependent source> --with-dependencies`.
**Actual:** the hint names an unknown flag.

## Evidence

- `docs/qa/evidence/2026-11-22-gh-1845-pr-b/cli/transcript.md`: entries "S7 enable human" and
  "probe enable --with-dependencies".

## Fix

**Disposition: fixed in this PR, pending re-walk.** Enable now prints `Enable first: compozy extension
enable <dep>.` for a disabled dependency and points a missing one to installing it or reinstalling
the dependent with `--with-dependencies`. Test: "Should guide enable without the install-only
--with-dependencies flag".

**Disposition: not fixed here (QA walk).** The CLI renders the same install-oriented hint for
`extension_dependencies_required` from install and from enable
(`internal/cli/extension_dependencies.go`).

## Verification

Pending fix.
