# Release QA Verification Report

## Scope

Release-grade QA for `/Users/pedronauck/Dev/compozy/looper`, covering the full verification gate, Codex default model behavior, docs/config consistency, daemon runtime, embedded web UI, and a temporary Node.js API workspace.

## Result

Pass after fixes.

## Fixes Applied

- Changed the built-in Codex default model from `gpt-5.4` to `gpt-5.5`.
- Updated the Codex ACP bootstrap so `codex-acp` receives `-c model="gpt-5.5"` at process start instead of relying only on post-session model switching.
- Updated active docs, help fixtures, config examples, tests, and generated expectations from non-mini `gpt-5.4` to `gpt-5.5`.
- Restored production `Disket Mono` font-face declarations in `packages/ui/src/tokens.css`.
- Hardened executor ACP integration tests that launch the real helper process so full-suite `-race` load does not time out before the intended ACP scenario begins.

## Canonical Verification

- `make verify` passed.
- Frontend lint/typecheck/test/build passed.
- Go fmt, golangci-lint, race tests, and build passed.
- Playwright daemon-served web UI E2E passed: `5` tests.
- Go test total reported by the gate: `2739` tests, `2` expected helper-process skips, `0` failures.

## Focused Regression Checks

- `bun run --cwd packages/ui test` passed: `5` files, `22` tests.
- `go test -race ./internal/core/run/executor -count=1` passed.
- `go test ./internal/core/agent -run TestCodexFallbackLaunchBootstrapsDefaultModel -count=1 -v` passed.
- `rg -n -P 'gpt-5\\.4(?!-mini)' README.md docs skills internal web packages test .github Makefile` returned no active non-mini `gpt-5.4` references.

## Temporary Node.js API Smoke

Fixture path: `/tmp/compozy-release-node-api-lR1dx3`

- `npm test` passed: `1` Node test, `0` failures.
- `compozy tasks validate --name release-api --format json` passed: `1` task scanned, `0` issues.
- `compozy tasks run release-api --dry-run --stream --ide codex` passed.
- Run result recorded `status=succeeded`, `ide=codex`, `model=gpt-5.5`.
- The dry-run prompt was generated under isolated home-scoped daemon artifacts:
  `/tmp/compozy-release-qa-home/.compozy/runs/tasks-release-api-1353e0-20260424-032224-000000000/jobs/task_01-c05bd2.prompt.md`

## Daemon and Web UI Smoke

Isolated daemon home: `/tmp/compozy-release-qa-home`

- Daemon reached `ready` with workspace `compozy-release-node-api-lR1dx3`.
- Workspace registry resolved `/tmp/compozy-release-node-api-lR1dx3`.
- Browser-use validated the daemon-served UI at `http://127.0.0.1:52551/`:
  - dashboard visible
  - active workspace displayed
  - workflow inventory showed `release-api`
  - sync action returned `Synced release-api — 1 task upserted.`
  - task board rendered `Validate Node Release API Smoke Fixture`
  - task detail route rendered the task and related run
- Screenshot evidence:
  `.codex/release-qa-2026-04-23/qa/screenshots/daemon-node-api-task-detail.png`
- The isolated daemon was stopped after the browser smoke.

## Issues Found

- `BUG-001`: UI token package omitted Disket Mono font faces. Fixed and verified.
- `QA-NOTE-001`: Repository-local `scripts/discover-project-contract.py` is absent. Documented; release gate discovery used repository Makefile/package/Playwright contracts.

## Residual Risk

- Live ACP execution against the real hosted `gpt-5.5` model was not performed; the release-safe validation used dry-run/task prompt generation plus ACP bootstrap command tests. This avoids external model cost and side effects while proving the configured default and command bootstrap path.
- The `gpt-5.4-mini` references remain intentionally unchanged because the request was to replace the non-mini Codex default.
