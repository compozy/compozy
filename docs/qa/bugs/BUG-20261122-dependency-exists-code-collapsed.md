# BUG-20261122-dependency-exists-code-collapsed: A dependency's extension_exists refusal is reported as extension_install_failed

- **Status:** open
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

**Disposition: not fixed here (QA walk).** The dependency error wrapping should keep the inner
acquisition code (`extension_exists`) and add `dependency`, the same way it already does for
`extension_checksum_unverified` and `extension_gateway_confirmation_required` (steps 2 and 3).

## Verification

Pending fix.
