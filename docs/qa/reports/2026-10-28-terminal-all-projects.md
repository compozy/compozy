# QA Run Report — 2026-10-28 — Terminal under All projects

- Scope: issue #1008; terminal creation, attachment, bound-window closure, and project/User canaries.
- Cadence: targeted; Playwright drives the real production Web bundle and isolated daemons.
- Initial base: `7ccc00670d1e278515049d6ff7ba8b3c00b7ee25`.
- Status: focused terminal charter passed; PR CI and review remain separate delivery gates.

## Session matrix

| Persona | Charter | Scenario | Tour | Status |
|---|---|---|---|---|
| Marina, desktop, en-US | Terminal lifecycle below | ET-terminal-browser-lifecycle | Feature | Passed, focused slice |
| Lea, desktop, en-US | User terminal creation and closure | MS-global-scope-no-workspace-work | Feature | Passed, terminal slice |

## Reproduction and repairs

Before editing production code, All projects creation returned HTTP 403:

```json
{"error":{"code":"forbidden","message":"authorize terminal browser identity: window manager client unauthorized"}}
```

The viewer registered under `global`, but creation authorized its token against the target project.
The repair carries the exact registration partition through create and attach, retaining token and
profile validation. Real-browser validation also found two related incomplete paths: the attachment
component dropped this identity field, and a new tab dropped the originating window's workspace
binding. Both paths now preserve their source identity/binding.

The User canary exposed HTTP 500 after authorization: terminal environment resolution sent an empty
canonical workspace ID to the project resolver. Human User terminals now use `ResolveUser` for the
acting profile, consistent with their settings and working-directory resolution. The finding is
recorded in issue #1008.

## Session debriefs

All projects → project, All projects → User, and the User desktop each passed this real-browser walk:

1. Create a terminal, verify HTTP 201 and the viewer's registration partition.
2. Attach interactively and execute a shell command; independently read its output through the public API.
3. Open a second tab from the bound window, attach and execute another command in the same destination.
4. Cancel the running group-close confirmation; verify both processes remain running through the API.
5. Confirm close; verify both processes stop and their managed windows disappear.
6. Create a persisted bound window referencing a nonexistent terminal through the public window-manager API.
7. Close that window without confirmation/error, then reload and verify no terminal windows return.

The existing project-desktop lifecycle canary also passed: two tabs, live input, reload/reattach,
retained output, cancel/confirm group close, and retained history. No operator daemon or windows
were modified. Test fixtures own teardown of their isolated processes; the initial reproduction lab
uses the canonical bootstrap manifest and teardown helper.

## Verification

- Existing API browser-identity suite: create/attach with real window-manager registration, exact partition,
  forged/mismatched tokens, and revoked-token rejection passed.
- Existing OpenAPI suite: both optional browser identity headers are published on create/attach.
- Existing daemon provider-vault suite: declaration-free terminal environment resolves for project,
  canonical User, and `@user`; passed with the same profile resolver wiring as daemon startup.
- Existing Web adapter/close suites: 47 tests passed, including mixed project/User window bindings
  while the desktop is All projects or another project, and missing-terminal closure.
- Root Turborepo Web lint/typecheck passed with zero lint warnings/errors.
- Root Turborepo focused Playwright run: four tests passed in 51.4 seconds. Only `terminal.spec.ts`
  was selected; no full local E2E or race lane was run.
- Production Web build and daemon build passed. Web build reports existing CSS/chunk diagnostics.
- React Doctor reported 100/100; no issues.
- Owning code generation completed for the OpenAPI document and TypeScript/Go SDK contracts.
  The baseline full codegen check found unrelated open-design embedded-linter lexical-name drift;
  that unrelated generated edit is excluded. CI owns the complete generated-artifact check.

Screenshots from the final run are named `global-project-attached`, `global-project-confirm`,
`global-project-closed`, `global-project-missing`, `global-project-missing-closed`, and corresponding
`global-user`/`user` captures. The PR links the published R2 copies. The initial launcher capture is
paired with the exact 403 response above; it does not independently show the transient error toast.

## Limits

This verifies the changed terminal slices, not every journey in the two broader scenario files.
Their existing aggregate QA status is preserved. Required current-head CI and both external reviews
must pass before delivery; local focused evidence alone does not establish that status.
