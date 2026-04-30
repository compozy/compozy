Goal (incl. success criteria):

- Run release-grade QA using `qa-report` and `qa-execution`, including docs/config review, daemon/runtime smoke, web UI browser validation, and a temp Node.js API project flow.
- Change Codex default model from `gpt-5.4` to `gpt-5.5` and verify it works.
- Success means production code/config/docs are fixed as needed, QA artifacts are written, runtime/browser scenarios are exercised, and `make verify` passes after all changes.
- Current follow-up: create a fresh temporary Node.js application with PRD, TechSpec, and task files, then validate Compozy E2E through CLI, daemon, and Web UI.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE instructions; no destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit user permission.
- Use required skills before code/test/final claims: `qa-report`, `qa-execution`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `cy-final-verify`.
- Do not touch unrelated dirty/untracked work unless needed for this task.
- Local date from shell is `2026-04-23`.

Key decisions:

- Use `.codex/release-qa-2026-04-23` as the shared QA output path for report and execution artifacts.
- Treat prior `.codex/ledger/2026-04-24-MEMORY-release-hardening.md` as read-only context; it reports previous daemon/CI/docs work and a `make verify` failure in frontend token tests due to pre-existing `packages/ui/src/tokens.css` changes.

State:

- Follow-up E2E completed; final repository `make verify` passed after the temp Node.js fixture, daemon, and Web UI validation.

Done:

- Read `qa-report`, `qa-execution`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, and `cy-final-verify` skill instructions.
- Scanned `.codex/ledger/*-MEMORY-*.md` filenames and searched for Codex/model/daemon/web context.
- Read prior release-hardening ledger.
- Discovered canonical gate from `Makefile`: `make verify` runs frontend verify, Go fmt/lint/race tests/build, and `frontend-e2e`.
- Discovered Web UI E2E harness: `web/playwright.config.ts` and `web/e2e/daemon-ui.smoke.spec.ts`.
- `qa-execution` repo-local discovery command `python3 scripts/discover-project-contract.py --root .` failed because that file is absent; using skill reference + Makefile/package/CI discovery instead.
- Created QA artifacts under `.codex/release-qa-2026-04-23/qa/`: release test plan, regression suite, and P0/P1 test cases.
- Checked Codex ACP dependency surface: local `codex-acp` absent; `npx --yes @zed-industries/codex-acp --help` supports `-c key=value`; npm latest is `0.11.1`.
- Changed `model.DefaultCodexModel` to `gpt-5.5`.
- Updated Codex registry to mark Codex as bootstrap-model capable and pass `-c model="<model>"` to `codex-acp`.
- Updated active docs/help/examples/tests from `gpt-5.4` to `gpt-5.5` where applicable.
- Focused tests passed:
  - `go test ./internal/core/agent -run 'Test(AgentRegistryEntries|CodexFallbackLaunchBootstrapsDefaultModel|DriverCatalogCanonicalCommandExcludesDynamicBootstrapArgs|BuildShellCommandStringUsesFallbackLauncherWhenPrimaryMissing|ResolveLaunchCommandUsesFallbackCandidate)' -count=1`
  - `go test ./internal/core/model ./internal/core/agents ./internal/cli -run 'Test(RuntimeConfigRuntimeForTask|Documented|BuildInspectAgentReportAndListHelpers|NewRunWorkflowDispatchesStartCommand|BuildConfig|TaskRuntime|DaemonDocs|ReusableAgents|ApplyWorkspaceDefaults)' -count=1`
  - `go test ./internal/api/client ./internal/api/contract ./internal/core/modelprovider ./internal/core/workspace -count=1`
- First `make verify` failed in `frontend:test`: `packages/ui/tests/tokens.test.ts` expected Disket Mono font faces in `packages/ui/src/tokens.css`.
- Fixed production UI tokens by restoring Disket Mono regular/bold `@font-face` declarations that point at existing packaged font assets.
- Focused UI test passed: `bun run --cwd packages/ui test` (`5` files, `22` tests).
- Second `make verify` reached Go race tests and failed in two ACP executor integration happy paths because the helper process missed a 3s initialization/activity deadline under full-suite load.
- Reproduced those two tests passing in isolation with and without `-race`; full `go test -race ./internal/core/run/executor -count=1 -v` also passed locally.
- Hardened the ACP helper happy-path tests by using a 10s integration timeout while keeping the same output/session assertions.
- Focused race test passed: `go test -race ./internal/core/run/executor -run 'TestExecuteJobWithTimeoutACPFullPipelineRoutesTypedBlocks|TestExecuteJobWithTimeoutACPCycleBlockKeepsParentSessionUsable' -count=1`.
- Third `make verify` failed in `TestJobRunnerACPErrorThenSuccessRetries` for the same helper-startup timing root cause before the intended first prompt error reached the err log.
- Applied the same 10s helper timeout to the remaining executor integration tests that launch the real ACP helper process with explicit 1s runtime timeouts.
- Full executor race package passed: `go test -race ./internal/core/run/executor -count=1`.
- `make verify` passed end-to-end after fixes: frontend bootstrap/lint/typecheck/test/build, Go fmt/lint/race tests/build, and Playwright daemon UI E2E (`5 passed`).
- Temporary Node API fixture created at `/tmp/compozy-release-node-api-lR1dx3`.
- Node fixture passed `npm test`.
- Task validation initially caught a fixture metadata title/H1 mismatch; fixed the fixture and validation passed.
- `compozy tasks run release-api --dry-run --stream --ide codex` passed under isolated daemon home `/tmp/compozy-release-qa-home`; result recorded `model=gpt-5.5`.
- Daemon reached ready on localhost HTTP port `52551`, workspace resolved/listed correctly, and was stopped after smoke.
- Browser-use validated daemon-served dashboard, workflows, sync, task board, and task detail for the temp Node API workspace.
- Wrote screenshot evidence: `.codex/release-qa-2026-04-23/qa/screenshots/daemon-node-api-task-detail.png`.
- Wrote issue notes and report: `BUG-001.md`, `QA-NOTE-001.md`, `verification-report.md`.

Now:

- Ready to report follow-up E2E evidence.

Next:

- Await user review or next release action.

Open questions (UNCONFIRMED if needed):

- None yet.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-23-MEMORY-release-qa.md`
- `.codex/release-qa-2026-04-23/qa/`
- Follow-up QA output path: `.codex/release-qa-2026-04-24/qa/` (planned)
- Follow-up temp project: `/tmp/compozy-node-e2e-E0vmvw`.
- Follow-up isolated daemon home: `/tmp/compozy-node-e2e-home-E0vmvw`.
- Follow-up run ID: `tasks-task-ledger-api-b7ec25-20260424-033157-000000000`.
- Follow-up browser URL: `http://127.0.0.1:52960/` during validation; daemon stopped afterward.
- Follow-up screenshot: `.codex/release-qa-2026-04-24/qa/screenshots/task-ledger-runs.png`.
- Follow-up report: `.codex/release-qa-2026-04-24/qa/verification-report.md`.
- Commands used: `date +%F`, `find .codex/ledger ...`, `rg ... .codex/ledger`, `git status --short --branch`, `python3 scripts/discover-project-contract.py --root .` (failed missing file).
