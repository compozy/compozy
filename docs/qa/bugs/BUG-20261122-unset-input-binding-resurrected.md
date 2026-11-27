# BUG-20261122-unset-input-binding-resurrected: An unbound extension input value comes back on the next setup write or reinstall

- **Status:** verified (re-walk 2026-11-22, `331c6f564`)
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

**Root cause.** A dual-consumer non-secret input (`region`: subprocess via `requires_env` and an MCP
server) stores its value twice: the canonical env binding plus a typed row for MCP publication
(spec rule 3). Three paths ignored that:

- `secrets unset` (`deleteExtensionSecretForProfileInstance`, `internal/daemon/extension_secrets.go`)
  deleted only the binding. The active typed row kept the reader reporting `set`
  (`extensionInputReader.load`, `internal/daemon/extension_inputs_state.go`).
- On the next write, `carriedSubprocessValues` (`internal/daemon/extension_inputs_plan.go`), the F-1
  upgrade carry-forward, saw an active row with no active binding and minted the binding again.
- Removal retired the bindings and Vault values but left the typed row. A reinstall's `Prepare`
  carried it forward the same way.
- The inflated count after `unset` then `secrets set` came from `secrets set` writing the new binding
  without its `input_id`, because the prior binding no longer existed. The next input write re-tagged
  it (`inputSecretUnchanged` false) and counted the input as changed. A row write that only mirrors
  the canonical binding was also counted.

**Fix.**

- `secrets unset` of an input's env binding also deletes that input's typed row and restores it if
  the unset fails (`unsetInputRow`). The input reads `daemon_env` or `missing` (rules 5a/5c), and
  carry-forward has nothing to resurrect.
- `secrets set` links a binding to the declared secret or subprocess input bound to that env name
  (`envBindingInputID`), so a later write leaves it untouched.
- `plannedInputChanges` (`internal/daemon/extension_setup.go`) no longer counts row writes of
  subprocess inputs. Their binding is canonical (D2), so only binding writes change a value.
- Removal (`retireExtensionBindingScopes` → `retireSubprocessInputRows`,
  `internal/daemon/extension_secret_retirement.go`) deletes the typed rows that mirror retiring input
  bindings in the same cells. Compensation restores them with the bindings.
- F-1 carry-forward is unchanged: a typed row whose input never had a binding is still minted into
  one.

**Scope note (pre-existing, not changed).** On `main`, removal already leaves the typed rows of
inputs that have no input binding: MCP-only inputs, and dual-consumer rows written before the input
gained a subprocess target and never re-bound. A reinstall in the same cell reuses those rows for MCP
publication. This predates #1845 and is left as is. For a dual-consumer row in that state, the F-1
carry-forward would bind it on reinstall.

**Tests.** `TestExtensionInputBinderSubprocessInputs` in `internal/daemon/extension_secrets_test.go`:

- "Should keep an unset dual-consumer input unset across later writes" covers unset staying unset,
  a later write changing only the named input, and `unset` then `secrets set` counting only the
  named input.
- "Should discard subprocess input values on removal so a reinstall starts from defaults" covers
  removal, a reinstall refusing the region value as missing, and compensation restoring the row.
- The existing "Should carry a typed row forward into the binding of a newly subprocess-bound
  input" keeps F-1 covered.

## Verification

Re-walk 2026-11-22: isolated `eng-qa-bootstrap` lab (fresh home, HTTP 40371, own UDS,
`[automation] enabled = false`). Binary built from `331c6f564` stamped `v0.7.9`. The daemon ran
with `REGION` and a canary `ACME_TOKEN` in its own environment. Fixture: same `acme-tool` as the
original walk. Evidence: docs/qa/evidence/2026-11-22-gh-1845-pr-a-rewalk/cli/.

- Repro steps 1–4 (r1-repro.txt): pass. `compozy__extensions_setup` wrote `region`, and the probe
  read `REGION_VALUE=us-east-2`. `secrets unset --env REGION` → `bound_env_keys` `[ACME_TOKEN]`.
  Setup read `region` `daemon_env`, not `set`. `setup --input project=false` printed
  `Updated 1 input of acme-tool. Restarted acme-tool.` and left `REGION` unbound. The restarted
  subprocess logged `R1=absent`.
- Repro step 5 (r1-repro.txt): pass. Remove left `vault list` empty. A reinstall without a region
  value created only `env/ACME_TOKEN`. `region` read `daemon_env`, and the probe recorded an empty
  `REGION_VALUE`.
- Removal of a bound region (r2-remove-bound.txt): pass. After `setup --input region=ap-south-1`
  (probe `REGION_VALUE=ap-south-1`), remove and reinstall left `region` `daemon_env`, `REGION`
  unbound, and the probe value empty.
- Count after `unset` then `secrets set` (r3-unset-set-count.txt): pass. After
  `secrets unset --env ACME_TOKEN` and `secrets set --value-stdin`, `api_token` read `set`. The next
  `setup --input project=true` printed `Updated 1 input`, and the restarted subprocess had
  `A1=present`.
- F-1 upgrade carry-forward (r9-f1-upgrade.txt): pass. With v0.1.0, where `REGION` was not in
  `requires_env`, a stored `region=sa-east-1` gave `bound_env_keys` `[ACME_TOKEN]` and
  `R1=absent`. `install --replace` with v0.3.0, which adds `REGION`, bound it
  (`[ACME_TOKEN, REGION]`) without re-entry. The probe read `REGION_VALUE=sa-east-1`.
- No entered token or the daemon canary appears in CLI transcripts, the daemon log or runtime logs.
- New, separate defect found while restarting a workspace instance:
  BUG-20261122-workspace-enable-reads-global-inputs.

**Status after re-walk:** verified fixed.
