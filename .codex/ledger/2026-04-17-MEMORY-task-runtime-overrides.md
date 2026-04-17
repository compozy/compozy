## Goal (incl. success criteria):

- Make the per-task runtime feature production-grade for extensions.
- Success criteria:
- Add an official early extension seam for PRD task runtime selection before derived job state is built.
- Reject late runtime mutation in planner and executor hooks.
- Guard `run.pre_start` against workflow config mutation after planning has consumed prepared state.
- Public SDK/docs/supported hook lists include the new seam.
- Full repository verification passes via `make verify`.

## Constraints/Assumptions:

- Follow workspace policies from `AGENTS.md` / `CLAUDE.md`, including ledger maintenance and non-destructive git handling.
- Required skills used for this session: `no-workarounds`, `systematic-debugging`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Keep the existing per-task runtime behavior intact; this change is about extension seams and guardrails, not the user-facing CLI/TUI feature itself.
- Scope of the new early hook is `prd-tasks` only.

## Key decisions:

- The root cause is phase mismatch: task runtime is resolved during planning, but current mutable hooks that can change runtime are too late in the pipeline.
- Add `plan.pre_resolve_task_runtime` as the official extension seam instead of letting extensions rewrite `TaskRuntimeRules` or mutate jobs later.
- Freeze runtime mutation in `plan.post_prepare_jobs` and `job.pre_execute`.
- Add workflow prepared-state guards to `run.pre_start` instead of relying on silent ignores.
- Validate the runtime produced by the new hook through the existing runtime validation path.

## State:

- Completed and verified.

## Done:

- Persisted the accepted production-grade follow-up plan in `.codex/plans/2026-04-17-task-runtime-overrides.md`.
- Re-read the planner, executor, SDK, and extension-manager flow to confirm exact edit points and root cause.
- Added the official early hook seam `plan.pre_resolve_task_runtime` for PRD tasks and wired it into planner runtime resolution before prompt/system/MCP derivation.
- Added shared `model.TaskRuntime` / `model.TaskRuntimeTask` types and public SDK mirrors.
- Rejected late runtime mutation in:
- `plan.post_prepare_jobs`
- `job.pre_execute`
- Added workflow prepared-state guards for `run.pre_start` with explicit stable errors for fields already consumed during planning.
- Updated executor hook runtime config conversion so late-mutable sound settings continue to propagate correctly.
- Updated public SDK hook types/builders/smoke coverage and extension manifest supported hooks for the new event.
- Updated extension hook reference docs to document the new seam and the new late-mutation constraints.
- Added regression tests covering:
- early runtime mutation through `plan.pre_resolve_task_runtime`
- `plan.post_prepare_jobs` runtime guard
- `job.pre_execute` runtime guard
- `run.pre_start` forbidden mutation rejection
- `run.pre_start` safe late-mutable fields still applying
- Ran targeted validation:
- `go test ./internal/core/plan ./internal/core/run/executor ./internal/core/extension ./sdk/extension -count=1`
- Ran full verification successfully:
- `make verify`
- Final result:
- `0 issues`
- `DONE 1934 tests in 47.501s`
- build succeeded
- final line `All verification checks passed`

## Now:

- Task complete.

## Next:

- None.

## Open questions (UNCONFIRMED if needed):

- None.

## Working set (files/ids/commands):

- `.codex/plans/2026-04-17-task-runtime-overrides.md`
- `.codex/ledger/2026-04-17-MEMORY-task-runtime-overrides.md`
- `internal/core/model/{runtime_config.go,preparation.go,task_runtime.go}`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/{execution.go,hooks.go,runner.go}`
- `internal/core/extension/{manifest.go,capability.go}`
- `sdk/extension/{types.go,hooks.go,handlers.go,smoke_test.go,compat_test.go}`
- `docs/extensibility/hook-reference.md`
- Commands:
- `go test ./internal/core/plan ./internal/core/run/executor ./internal/core/extension ./sdk/extension -count=1`
- `make verify`
