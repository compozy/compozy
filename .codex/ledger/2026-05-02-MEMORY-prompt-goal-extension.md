Goal (incl. success criteria):

- Implement the accepted QA workflow extension plan: automatic `$qa-report` and `$qa-execution` tasks, task-specific runtimes, `_tasks.md` index sync, and `/goal` as the first token for QA execution prompts.
- Activate the extension for `/Users/pedronauck/Dev/compozy/agh` and replace the manually-created final QA tasks in `agent-soul` with extension-owned generation.

Constraints/Assumptions:

- Follow workspace AGENTS/CLAUDE instructions.
- Never run destructive git commands without explicit user permission.
- Use local code/docs for repository behavior; no web search for local code.
- Accepted plan is persisted at `.codex/plans/2026-05-02-qa-workflow-extension.md`.
- QA output path default: `.compozy/tasks/<workflow>`.
- Claude xhigh is enforced best-effort through runtime config and `CLAUDE_CODE_EFFORT_LEVEL=xhigh`.

Key decisions:

- Use a Go extension because the Go SDK exposes `OnPlanPreResolveTaskRuntime`.
- Use `plan.pre_discover` for task creation, `plan.pre_resolve_task_runtime` for runtime patches, and `agent.pre_session_create` for final `/goal` prefixing.
- Extend `host.tasks.create` with `update_index` rather than having the extension patch `_tasks.md` directly.

State:

- Implementation complete; full verification passed.
- QA execution complete for the implemented QA workflow extension.
- Activating the extension in `../agh` and cleaning manual `agent-soul` QA tasks.

Done:

- Scanned existing ledgers for extension/hook context.
- Read the `compozy` skill overview.
- Confirmed extension capabilities include `prompt.mutate`, `job.mutate`, and `agent.mutate`.
- Confirmed `prompt.post_build` can replace rendered prompt text and exposes `batch_params.batch_groups`.
- Confirmed `job.pre_execute` can mutate `Job.Prompt`/`Job.SystemPrompt` immediately before execution and exposes `Job.CodeFiles`, `Job.Groups`, and task metadata.
- Confirmed `agent.pre_session_create` can mutate the final ACP session prompt after system prompt composition, but has less task metadata beyond `job_id` and prompt text.
- Persisted accepted implementation plan under `.codex/plans/`.
- Added `update_index` to `host.tasks.create`, SDK Go, SDK TS, and Host API docs.
- Added `_tasks.md` append support owned by Host API.
- Added `extensions/cy-qa-workflow` with `plan.pre_discover`, `plan.pre_resolve_task_runtime`, and `agent.pre_session_create` handlers.
- Added focused Host API, SDK Go, and extension unit tests.
- Ran focused Go tests for `internal/core/extension`, `sdk/extension`, and `extensions/cy-qa-workflow`: passed.
- Ran `bun run --cwd sdk/extension-sdk-ts typecheck`: passed.
- Ran `go vet ./...`: passed.
- First `make verify` failed on lint; root cause was local structural lint in new code (function complexity, range copy, long lines).
- Refactored without suppressions; ran focused lint and tests: passed.
- Ran final `make verify`: passed. Evidence: frontend checks passed, golangci-lint reported `0 issues`, gotestsum reported `DONE 3021 tests, 3 skipped`, Go build succeeded, Playwright E2E reported `5 passed`, and Make printed `All verification checks passed`.
- Started `$qa-execution` for the extension. The skill discovery script path `scripts/discover-project-contract.py` is absent in this repo, so QA uses documented repo signals (`AGENTS.md`, `Makefile`, `web/package.json`, Playwright config).
- Ran baseline `make verify`: passed. Evidence: frontend checks passed, `golangci-lint` reported `0 issues`, gotestsum reported `DONE 3021 tests, 3 skipped`, Go build succeeded, Playwright E2E reported `5 passed`, and Make printed `All verification checks passed`.
- Exercised the public CLI flow in `.codex/tmp/qa-workflow-extension-lab` with isolated HOME `/tmp/cqawf-home-20260502012333`: workspace extension discovered/enabled, setup installed Codex and Claude Code skills, `tasks validate` passed, and `tasks run qa-ext-smoke --dry-run --stream` created QA tasks and completed 3 jobs.
- Reran the same `tasks run` command and confirmed idempotency: only `task_01.md`, `task_02.md`, and `task_03.md` exist; `_tasks.md` has exactly the original task plus QA report and QA execution rows.
- Confirmed generated runtime metadata from run artifacts: task_02 uses `claude` + `opus` + `xhigh`; task_03 uses `codex` + `gpt-5.5` + `xhigh`.
- Ran focused extension tests for task creation/runtime/session prompt mutation: passed.
- Exercised daemon Web UI with `agent-browser` against the isolated daemon at `http://127.0.0.1:62544`: dashboard, workflow board, task detail, and run detail rendered; screenshots stored under `.codex/qa/qa-workflow-extension/qa/screenshots/`.
- Ran final `make verify` after QA scenarios: passed with 0 lint issues, 3021 Go tests (3 skipped), successful build, 5 Playwright E2E tests, and `All verification checks passed`.
- Reran the critical CLI dry-run after final verification with the rebuilt binary: passed, 3 jobs completed, exactly 3 task files remained.
- Wrote QA report and bootstrap manifest under `.codex/qa/qa-workflow-extension/qa/`.

Now:

- Inspecting `../agh` instructions, existing extension state, and `agent-soul` task files before modifying artifacts.

Next:

- Remove only the requested final manual QA tasks and enable `cy-qa-workflow` for `../agh`.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-05-02-MEMORY-prompt-goal-extension.md`
- `.codex/plans/2026-05-02-qa-workflow-extension.md`
- `.agents/skills/compozy/SKILL.md`
- `.agents/skills/golang-pro/SKILL.md`
- `.agents/skills/testing-anti-patterns/SKILL.md`
- `.codex/ledger/2026-04-10-MEMORY-extension-foundation.md`
- `.codex/ledger/2026-04-10-MEMORY-hook-dispatches.md`
- `.codex/ledger/2026-04-10-MEMORY-job-run-review-hooks.md`
- `sdk/extension/hooks.go`
- `sdk/extension/types.go`
- `sdk/extension/host_api.go`
- `sdk/extension-sdk-ts/src/types.ts`
- `sdk/extension-sdk-ts/templates/prompt-decorator/`
- `docs/extensibility/host-api-reference.md`
- `internal/core/prompt/common.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/runner.go`
- `internal/core/run/internal/acpshared/command_io.go`
- `internal/core/extension/host_writes.go`
- `extensions/cy-qa-workflow/`
- `.agents/skills/qa-execution/SKILL.md`
- `.codex/qa/qa-workflow-extension/`
- `.codex/qa/qa-workflow-extension/qa/verification-report.md`
- `.codex/qa/qa-workflow-extension/qa/bootstrap-manifest.json`
- `.codex/tmp/qa-workflow-extension-lab/`
- `/tmp/cqawf-home-20260502012333`
- `make verify`
- `HOME=/tmp/cqawf-home-20260502012333 ... bin/compozy tasks run qa-ext-smoke --dry-run --stream`
- `/Users/pedronauck/Dev/compozy/agh`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/agent-soul`
