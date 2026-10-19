# Issue #832: session metadata and aggregate folders

## Scope and charter

Bruno manages two profiles in one workspace. He identifies every session by runtime and agent,
organizes work in folders, switches between a profile and All profiles, renames a folder belonging
to the other profile, and reloads to confirm saved membership. Only targeted checks and isolated
runtime/browser journeys are authorized; CI owns global delivery gates.

## Session matrix

These results cover the issue #832 extensions, not a rerun of every historical scenario step.

| Scenario | Status | Evidence |
| --- | --- | --- |
| ET-web-session-folders-manual-order | Pass | E2E-032: single-profile and aggregate membership, foreign drop ignored, rename persisted after reload; canonical folder suites cover ordering/collapse |
| ET-profile-web-aggregate-owner-surfaces | Pass | E2E-032: each profile retains its folder group and mutation destination |
| ET-profile-aggregate-owner-labels | Pass | E2E-032: owner groups remain visible in All profiles |
| RT-session-list-row-actions | Pass | Real Codex root and never-bound child retain provider/agent marks; idle release and daemon shutdown/restart preserve last-known runtime |

## Lab and runtime evidence

Manifest: `/Users/pedronauck/dev/qa-labs/compozy-issue-832-session-list-20261019-202236-484729-lab/qa-artifacts/qa/bootstrap-manifest.json`.
The lab used a separate COMPOZY_HOME, socket and HTTP port 64869; the user's daemon was not used.
The focused Playwright fixture also owns an independent ephemeral daemon/home.

Codex `gpt-6.1-sol`, reasoning `low`, returned `QA_OK` for session `sess-7ef1cabf736d9bb0`.
The public API and browser established:

- Active/ready: `effective` and `last_known` name the actual provider/model.
- Never-bound child `sess-991e9ab677158f4e`: provider/agent remain visible without a prompt.
- Idle release: lifecycle stays `active`, runtime becomes `unbound`, `effective` is absent,
  and `last_known` retains Codex, model and reasoning. The runtime process detached naturally.
- After orderly daemon shutdown/restart: lifecycle is `stopped`, runtime remains `unbound`,
  and both root and child rows retain metadata. No synthetic database state was injected.

Raw receipts under the manifest's `qa/` directory: `prompt.json`, `never-bound.json`, `child.json`,
`active-runtime.json`, `released-runtime.json`, and `detached-runtime.json` (the last records the
actual stopped/unbound state after restart). Browser captures are alongside those receipts.

## Checks

- `CGO_ENABLED=1 go test -race ./internal/api/core -run '^TestSessionPayloadFromInfo$' -count=1`: passed.
- `make codegen`: passed; OpenAPI and generated TypeScript/Go clients co-ship.
- Root Turbo, affected Web suites (`session-list`, `session-folders`, `use-session-folders`): 70 tests passed after rebase.
- `bunx turbo run build --filter=./web`: passed after rebase, including Web typecheck.
- Focused Playwright `profiles.spec.ts --grep E2E-032`: passed with the real daemon and no mock agent.
- Changed-file Oxlint and formatting: passed. React Doctor local scan found no issues; external score was not requested after approval review rejected external transmission.
- Existing build warnings concern CSS `::highlight`, chunk size and the terminal import split. The existing keyboard menu unit test emits React `act` warnings.
- QA evidence audit: PASS (zero blockers/warnings). Manifest teardown returned `clean=true`; its owned-process sweep was needed after graceful shutdown timed out. Global local gates were explicitly prohibited; CI remains the delivery gate.

## Screenshots

- [Single profile folders](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-832/single-profile-folders.png)
- [All profiles with owner folders](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-832/all-profile-folders.png)
- [Stopped root and child with retained runtime](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-832/stopped-runtime.png)

## Review remediation

Greptile findings #4318697437 and #4318697462 are addressed: aggregate navigation is gated while
catalog paging is pending, and an empty profile retains its folder-read error and Retry control.
The existing list suite now has 54 passing tests, including both paging states and a failed read
followed by a successful retry. Affected Web typecheck/build and E2E-032 passed again.

The first CI run found an unregistered additive field in the historical OpenAPI compatibility
baseline. The existing approved-addition adapter now adds only `last_known`, using the historical
`effective` runtime-selection schema at each inline site. All existing schema comparisons remain.
`CGO_ENABLED=1 go test -race ./internal/api/spec -run '^TestDocumentPreservesSchemaSemantics$' -count=1`
passed.

CodeRabbit findings #4318833230, #4318833236 and #4318833241 are addressed. Aggregate empty
search has a message and pending paging exposes `aria-busy`. Profile loading/error/retry are
visible above the retained catalog. Grouping activates only when every thread root has an owner
in the independent profile snapshot; otherwise rendering, bulk selection and keyboard cycling all
retain the flat catalog. The three owning suites pass 78 tests. React Doctor's local scan reports
no issues after keeping aggregate presentation and status notices in small components.

An accidentally unfiltered Web test invocation during this round was stopped by terminating only
its owned processes. It is not counted as validation evidence; the authorized three-file command
was rerun. No global gate or full E2E run was used.

## Final status

Targeted QA passed. PR delivery remains conditional on current-head required CI and reviewer feedback.
