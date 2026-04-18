# Task Memory: task_15.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Migrate review fetch/list/show/fix and ad hoc exec flows onto daemon-backed lifecycle ownership without changing the authored review Markdown contract or the user-facing exec prompt/output contract.

## Important Decisions
- Reuse `internal/daemon.RunManager` plus daemon transport/client layers instead of adding new local review/exec execution paths.
- Keep existing top-level `fetch-reviews` and `fix-reviews` command surfaces working while introducing the daemon-backed `reviews` command family required by the TechSpec.
- Preserve current CLI-owned prompt-source resolution and JSON/raw-JSON error emission for `exec`; daemon start requests should receive already-resolved prompt text.
- Treat task completion for this pass as proving the existing daemon-backed review/exec surfaces with focused unit and integration coverage rather than widening scope beyond task 15.

## Learnings
- The daemon-backed review/exec transport and CLI surfaces were already present in the worktree; the missing task-15 gap was verification depth around review sync, exec output compatibility, and the new client/transport seams.
- Exact statement coverage from the refreshed coverprofiles is now above the task target for the migration seams: `internal/cli/reviews_exec_daemon.go` 81.4%, `internal/daemon/review_exec_transport_service.go` 83.5%, and `internal/api/client/reviews_exec.go` 80.7%.
- `TestExecCommandUnknownAgentReturnsActionableError` needed explicit daemon-home/XDG isolation to stay stable under the full parallel `internal/cli` package run.

## Files / Surfaces
- `internal/api/client/reviews_exec_test.go`
- `internal/cli/{agents_commands_test.go,reviews_exec_daemon_additional_test.go}`
- `internal/daemon/review_exec_transport_service_test.go`

## Errors / Corrections
- Full-package `internal/cli` verification initially flaked because the unknown-agent test inherited ambient daemon-home env under parallel package execution; fixing the test to set explicit `HOME`/XDG daemon paths removed the false green path.

## Ready for Next Run
- Task 15 is verified complete; the next daemon task can assume review/exec daemon surfaces are in place and covered, then build on them for task 16 follow-up cleanup.
