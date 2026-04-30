# Release Hardening QA Test Plan

## Executive Summary

Objective: validate the Compozy release candidate through repository-defined verification, public CLI/runtime flows, daemon HTTP/Web UI behavior, docs/config consistency, and the Codex default-model change from `gpt-5.4` to `gpt-5.5`.

Key risks:

| Risk                                                                | Probability | Impact | Mitigation                                                                                                |
| ------------------------------------------------------------------- | ----------- | ------ | --------------------------------------------------------------------------------------------------------- |
| Codex default model remains `gpt-5.4` in runtime, docs, or examples | High        | High   | Update source constant, tests, docs, help text, and run CLI dry-run/agent resolution checks               |
| Daemon-served web UI regresses despite unit tests                   | Medium      | High   | Run `make frontend-e2e`, live daemon smoke, and browser-use validation                                    |
| Release gate fails due frontend token/config drift                  | High        | High   | Establish baseline, diagnose root cause, fix production/config/test alignment rather than weakening tests |
| Docs/config examples diverge from actual CLI behavior               | Medium      | Medium | Search docs/config references and verify generated help/test fixtures                                     |
| Local runtime flows depend on unavailable live ACP credentials      | Medium      | Medium | Use `--dry-run`/local boundaries for deterministic proof; record live credentials blockers explicitly     |

## Scope

In scope:

- `make verify` full release gate: frontend lint/typecheck/test/build, Go format/lint/race tests/build, Playwright E2E.
- Codex and Droid default model behavior that uses `model.DefaultCodexModel`.
- CLI config/default precedence for `[defaults]`, `[exec]`, `[tasks.run]`, and explicit `--model`.
- Docs and bundled skill references that describe Codex defaults or config examples.
- Daemon lifecycle and workspace APIs in isolated temporary `HOME`/workspace.
- Daemon-served Web UI and browser validation against localhost.
- Temporary Node.js API project workflow fixture for realistic task/project handling.

Out of scope:

- Live execution against remote LLM providers when credentials or adapter availability are missing.
- Historical docs under `docs/plans/**` unless they are active release-facing documentation.
- Reverting or discarding unrelated worktree changes.

## Test Strategy

1. Discover repository contracts from `AGENTS.md`, `Makefile`, `package.json`, `web/package.json`, CI workflows, Playwright config, and active docs.
2. Generate and execute P0/P1 public-surface test cases before claiming release readiness.
3. Diagnose failures with root-cause evidence before editing.
4. Add or update automated coverage for release-critical behavior when the repository already has an appropriate harness.
5. Validate final state with `make verify` after the last code change, then rerun selected runtime/browser smoke flows.

## Automation Strategy

| Flow                           | Classification                                | Target                                     | Notes                                                                                           |
| ------------------------------ | --------------------------------------------- | ------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| Full release gate              | existing-e2e                                  | `make verify`                              | Repository canonical gate includes Playwright E2E                                               |
| Codex default model resolution | needs-e2e                                     | Go unit/integration                        | Update existing runtime/CLI tests to assert `gpt-5.5`                                           |
| Config/docs consistency        | existing-e2e                                  | TS config tests + Go doc fixtures          | Existing fixtures cover help and docs examples; add/update where mismatched                     |
| Daemon lifecycle/API           | existing-e2e                                  | Go tests + live CLI smoke                  | Use temp HOME and ephemeral HTTP port                                                           |
| Daemon Web UI                  | existing-e2e                                  | `bun run --cwd web test:e2e` + browser-use | Existing Playwright harness covers dashboard/workflows/reviews/runs/archive/start               |
| Temp Node.js API project       | manual-only unless existing CLI E2E covers it | CLI dry-run/live local boundary            | Use realistic temp project; avoid live LLM dependency unless credentials/adapters are available |

## Environment Requirements

- macOS local developer machine.
- Go version required by `go.mod` / Makefile.
- Bun version from `.bun-version` (`1.3.11`).
- Node available for temporary API project and browser tooling.
- Playwright Chromium installed or installable through `bunx playwright install chromium`.
- `agent-browser` and Browser Use plugin available for browser validation.

## Entry Criteria

- Workspace instructions and relevant skills loaded.
- QA artifact directory exists at `.codex/release-qa-2026-04-23/qa/`.
- No destructive git commands used.
- Current worktree state recorded before edits.

## Exit Criteria

- All P0 cases pass.
- At least 90% P1 cases pass, with any blocked case documented with exact missing prerequisite.
- No Critical or High unresolved bugs remain.
- `make verify` passes after the last code change.
- Browser evidence screenshots and verification report are written.
- Docs/config references to the Codex default match production behavior.

## Timeline And Deliverables

| Phase          | Deliverable                                                   |
| -------------- | ------------------------------------------------------------- |
| Planning       | Test plan, regression suite, and executable test cases        |
| Baseline       | First verification output and failure diagnosis               |
| Implementation | Root-cause fixes for Codex default/model/docs/config failures |
| Runtime QA     | CLI/daemon/temp project/browser evidence                      |
| Final          | Verification report and issue files if bugs are found         |
