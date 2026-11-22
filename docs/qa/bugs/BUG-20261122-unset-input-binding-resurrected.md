# BUG-20261122-unset-input-binding-resurrected: An unbound extension input value comes back on the next setup write or reinstall

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-extension-setup-inputs steps 5–7
- **Scenarios:** ET-extension-setup-inputs; ET-ext-secrets-binding
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-a-extension-inputs.md

## Summary

Bruno removes a stored extension value with `compozy extension secrets unset <ext> --env REGION`.
The CLI reports `✓ unbound REGION` and `secrets list` no longer lists it. Setup still shows the
input as `set`. The next setup write to an unrelated input says `Updated 2 inputs` and quietly
binds REGION again. The extension process keeps receiving the old value. Removing the workspace
installation and installing it again in the same workspace also delivers the old value, although
the install supplied no value for that input.

The same re-activation inflates the setup count after a secret is unbound and set again with
`secrets unset` followed by `secrets set`. The next one-input setup write reports `Updated 2 inputs`.

## Reproduction

- **Environment:** isolated `eng-qa-bootstrap` lab, binary built from `f9f47e42b` stamped `v0.7.9`,
  local fixture `acme-tool` (non-secret string input `region` bound to `REGION` in
  `requires_env` and referenced by stdio MCP server `acme`).

1. Install `acme-tool` in workspace `ws-a` and write `region` once. In the lab, the native tool
   `compozy__extensions_setup` wrote `us-east-2`. `compozy extension setup --input region=…` reaches
   the same state.
2. `compozy extension secrets unset acme-tool --env REGION --workspace ws-a` → `✓ unbound REGION`;
   `secrets list -o json` → `bound_env_keys` without `REGION`.
3. `compozy extension setup acme-tool --workspace ws-a -o json` → `region` state `set`. Expected:
   `daemon_env` or `missing`.
4. `compozy extension setup acme-tool --workspace ws-a --input project=false` → `Updated 2 inputs of
   acme-tool. Restarted acme-tool.` Then `secrets list` lists `REGION` again, and the extension log
   shows `R1=present`.
5. `compozy extension remove acme-tool --workspace ws-a`. `vault list` shows no acme refs. Then
   `compozy extension install <fixture> --workspace ws-a --input api_token=…`, with no region
   supplied, creates a new `env/REGION` Vault ref. The subprocess probe records `REGION_VALUE=us-east-2`.

**Expected:** an unset binding stays unset. Setup reports the input as `daemon_env`/`missing`, and a
later write changes only the inputs it names. Removing an installation discards its input values,
so a reinstall starts from defaults.
**Actual:** the unset value remains readable as `set`. The next write re-activates it and counts it
as updated. The value survives removal and is re-delivered to a fresh installation.

## Evidence

- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/defect-unset-resurrect.txt
- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/defect-region-secrets-list.json
- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/defect-vault-after-reinstall.txt
- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/defect-vault-after-remove.txt
- docs/qa/evidence/2026-11-22-gh-1845-pr-a/cli/es-count-repro.out (replace-only `secrets set`: count 1)
  and es-count-repro2.out (`unset` then `set`: count 2)
- Likely owner (not verified): spec rule 3 "Prepare re-activates the prior binding" for
  subprocess-bound non-secret inputs, which also treats an explicitly unset binding as prior state.

## Fix

## Verification
