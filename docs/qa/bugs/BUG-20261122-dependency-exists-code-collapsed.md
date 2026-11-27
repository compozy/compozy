# BUG-20261122-dependency-exists-code-collapsed: A dependency's extension_exists refusal is reported as extension_install_failed

- **Status:** verified
- **Impact (user-side):** Friction for agents and scripts that branch on the error code
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-extension-dependencies step 12
- **Scenarios:** ET-extension-dependencies
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies.md

## Summary

A dependency is installed only for another profile, and its source has since published new bytes.
Installing the dependent with `--with-dependencies` correctly refuses and names the dependency and
`compozy extension update`. The structured `code` is `extension_install_failed`, but the scenario
expects `extension_exists`, which a plain install of an existing extension returns.

## Reproduction

- **Environment:** isolated lab, binary from `08bd19a60` stamped `v0.7.9`.

1. `compozy profile create other`, then `compozy profile use default`.
2. `compozy extension install <acme-leaf> --allow-unverified --enable --yes --profile other`.
3. Add a file to the `acme-leaf` source folder so the package bytes change.
4. `compozy extension install <acme-mid> --allow-unverified --with-dependencies
   --allow-unverified-dependency acme-leaf --yes -o json`. `acme-mid` requires `acme-leaf`.

**Expected:** `code: "extension_exists"` with `dependency: "acme-leaf"`, nothing updated.
**Actual:** exit 65 with
`{"error":"dependency \"acme-leaf\": extension: extension already exists: acme-leaf 0.1.0 is installed for another scope or profile and its source now provides 0.1.0; dependencies are never updated, run `compozy extension update acme-leaf` first","code":"extension_install_failed","dependency":"acme-leaf"}`.
The other profile keeps its installed `0.1.0`. A plain duplicate install
(`compozy extension install <acme-base> …` twice) returns `code: "extension_exists"`.

Step 11 shows the same collapse: a direct API install without
`dependencies.<name>.expected_digest` returns 400 `extension_install_failed` instead of a validation
code. Its message names the dependency.

## Evidence

- `docs/qa/evidence/2026-11-22-gh-1845-pr-b/cli/transcript.md`: entries "S12 install acme-mid
  default w/ deps …", "S12 human", "probe plain extension_exists code" and "S11 API install without
  expected_digest".

## Fix

**Disposition: fixed in this PR, pending re-walk.** Dependency-attributed errors now go through
the normal error mapping and then gain `dependency`/`dependencies_installed`
(`internal/api/core/extensions_operation_errors.go`); a missing dependency `expected_digest` is a
validation response that keeps its field. The step 12 case is now refused earlier with state
`installed_elsewhere`. Tests: `TestExtensionDependencyHandlers`,
`TestExtensionDependencyValidationPayload`.

**Disposition: not fixed here (QA walk).** The dependency error wrapping should keep the inner
acquisition code (`extension_exists`) and add `dependency`, the same way it already does for
`extension_checksum_unverified` and `extension_gateway_confirmation_required` (steps 2 and 3).

## Verification

Verified 2026-11-22 on `0cb8b750d` (re-walk report
`docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md`):
- Step 12: `acme-p12dep` is installed only for profile `other`, and its source bytes have changed. The
  preview reports `state: installed_elsewhere` with
  `retry_command: compozy extension update acme-p12dep --scope global --profile other`. The install
  refuses with `extension_dependencies_required` and the same command. `acme-p12a`, listed first, was
  not installed, and `other` keeps 0.1.0.
- Commit-time collision: in two concurrent installs that share the missing `acme-racedep`, the loser
  returned `code: "extension_exists"`, `diagnostic.id: extension.exists`, `dependency: "acme-racedep"`.
- Step 11: a direct API install without `expected_digest` returns 400 with
  `issues[{field: "expected_digest"}]` and `dependency: "acme-design"`.
