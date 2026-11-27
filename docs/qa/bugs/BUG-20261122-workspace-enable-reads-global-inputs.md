# BUG-20261122-workspace-enable-reads-global-inputs: Re-enabling a workspace extension installation refuses a set required input as missing

- **Status:** open-upstream (pre-existing; filed as compozy/compozy#1960)
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, ET-ext-secrets-binding step 4 ("after restart")
- **Scenarios:** ET-ext-secrets-binding; ET-extension-setup-inputs
- **Found:** 2026-11-22 · **Report:** re-walk of BUG-20261122-unset-input-binding-resurrected (this file's Evidence)

## Summary

Bruno installs `acme-tool` in workspace `ws-b` with `--input api_token=…`. The instance runs, and
`setup --workspace ws-b` reports `api_token` `set`. He runs `compozy extension disable acme-tool`
(`Disabled in profile default.`), then `compozy extension enable acme-tool`. Enable refuses with
`extension_inputs_required` for `api_token`, so the extension stays disabled. No `secrets` command
ran first, so the token value is not the cause.

A related symptom: while a workspace installation is in `error` because its token was unbound,
`compozy extension disable acme-tool` also fails with `extension_inputs_required`. The disable is
rolled back because the reload re-initializes the broken instance.

## Reproduction

- **Environment:** isolated `eng-qa-bootstrap` lab, binary built from `331c6f564` stamped `v0.7.9`,
  fixture `acme-tool` from docs/qa/evidence/2026-11-22-gh-1845-pr-a-rewalk/fixture.

1. `compozy extension install <fixture> --workspace ws-b --input api_token=… --allow-unverified --yes`
   → active and healthy.
2. In `ws-b`: `compozy extension disable acme-tool` → `Disabled in profile default.`
3. `compozy extension enable acme-tool -o json` → exit 2,
   `{"inputs":["api_token"],"missing_env":["ACME_TOKEN"],"code":"extension_inputs_required",…}`.

**Expected:** enable reads the input state of the workspace installation it re-enables, finds
`api_token` set and starts the instance.
**Actual:** enable reports the bound token as missing and stays disabled. The only recovery is
remove and reinstall.

## Evidence

- docs/qa/evidence/2026-11-22-gh-1845-pr-a-rewalk/cli/r7-enable-control.txt (clean install, disable, enable refused)
- docs/qa/evidence/2026-11-22-gh-1845-pr-a-rewalk/cli/r5-sb4.txt (same after `secrets unset` + `secrets set`)
- docs/qa/evidence/2026-11-22-gh-1845-pr-a-rewalk/cli/r6-disable-missing.txt (disable refused while in error)
- Likely owner (not verified): `checkProfileEnablementInputs` in
  `internal/daemon/extension_inputs_status.go` loads input state with
  `extensioninput.Instance{Extension, ProfileID}` and no `WorkspaceID`, so it reads the global cell.
  `populateExtensionInputStatus` in the same file passes `ext.Status.WorkspaceID`. The function is
  unchanged from `main`, so the defect is likely pre-existing. It was not reproduced on a `main`
  build.

## Verification
