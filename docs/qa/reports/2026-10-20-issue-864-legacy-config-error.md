# QA Run Report — 2026-10-20 — Workspace config rejection

- Scope: issue #864, legacy workspace config rejection and valid-registration canary.
- Cadence: targeted.
- Build: working tree on `fix/issue-864-legacy-config-error`.
- Environment: isolated temporary home, HTTP listener and UDS; real daemon and Web.
- Status: PASS for the targeted real scenario; PR CI and bot reviews remain delivery gates.

## Session Matrix

| Charter | Persona | Journey / scenario | Tour | Status |
| --- | --- | --- | --- | --- |
| CH-workspace-registration-recovery, scoped to config refusal | Bruno, desktop, en-US | J-operate-workspace-context / MS-web-workspace-add-directory-browser | Garbage | PASS |

## Plan

Register a temporary project containing legacy CLI configuration over HTTP, UDS and CLI.
Confirm code, path, unique keys and remediation; confirm no registration survives rejection.
Use Add project to see the same failure while the draft remains editable. Rename only the temporary
legacy file, retry, and independently read the resulting workspace. After registration, introduce
the invalid file again and verify that detail resolution reports 422 while listing stays usable.

## Automated Evidence

- Focused config, workspace-handler and CLI error-rendering regressions passed with `-race`.
- Add-project component suite: 14 tests passed through root Turborepo.
- Root codegen and codegen-check passed.
- Heavy test/lint/build delivery gates are delegated to PR CI as instructed.

## Session Debrief

- HTTP and UDS rejected the real legacy file with 422 and identical structured payloads.
- CLI returned exit 78 with the same JSON details. The rejected registration left the catalog empty.
- Web Add project displayed the config path, unique unknown keys and remediation, preserving the name/root draft.
- Renaming only the temporary legacy file and retrying in the same dialog returned 201. An independent detail read returned 200.
- Reintroducing that file after registration made detail reads return 422; the catalog remained available with HTTP 200. The temporary file was then renamed back.
- Captured screenshots: `01-legacy-config-rejection.png` and `02-registration-recovered.png`; public evidence links are in the PR.
- Browser validation used a separate headless Chromium profile, with real network requests and no mocks. The host daemon and user project config were untouched.
- No provider-backed sessions were needed for this workspace-registration contract.
- Full heavy gates remain delegated to CI; two pre-existing test-shape warnings in unrelated config tests were left unchanged.

## Review Remediation Re-walk

- Removed loop runtime, provider model and extension marketplace keys each returned 422 from the real daemon, retaining their specific replacement advice and typed key metadata.
- A Go SDK workspace call against that daemon preserved status, stable code, config path and unknown keys in `APIError`.
- Repeated the browser refusal/retry with another temporary project and a unique display name: 422 followed by 201, with the editable draft preserved.
- Re-ran the 14-test Web suite through root Turborepo, codegen-check, targeted config/core/CLI and SDK race tests, and product-language validation successfully.
- CI findings also required registering the optional error fields in the existing OpenAPI compatibility test and correcting lint conventions.
