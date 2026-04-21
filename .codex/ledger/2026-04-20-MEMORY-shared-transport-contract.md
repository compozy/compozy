Goal (incl. success criteria):

- Implement task `task_03.md` (`Shared Transport Contract Migration`) in the `daemon-improvs` worktree.
- Success means `internal/api/core`, `internal/api/httpapi`, and `internal/api/udsapi` emit canonical `internal/api/contract` payloads, errors, and SSE frames with HTTP/UDS parity across the current route inventory, backed by focused tests and clean `make verify`.

Constraints/Assumptions:

- Work only in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`.
- Must follow `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- Must read and update `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_03.md}`.
- Keep the current transport-facing service split intact; do not introduce a new facade above `TaskService`, `ReviewService`, `RunService`, `WorkspaceService`, `SyncService`, or `ExecService`.
- No destructive git commands without explicit user permission.
- Commit only task-related changes after verification; avoid staging unrelated tracking changes already present in the worktree.

Key decisions:

- The task belongs to the `daemon-improvs` worktree, not `/Users/pedronauck/dev/compozy/agh`; that checkout is unrelated dirty state and not the source of truth for this task.
- Use `.compozy/tasks/daemon-improvs/_techspec.md`, `task_03.md`, ADR-001, and ADR-003 as the source of truth for scope and validation.
- Treat `internal/api/core/routes.go` as the authoritative current route inventory that HTTP and UDS must keep in parity.

State:

- Completed with verified code changes, tracking updates, and a scoped local commit.

Done:

- Read root `AGENTS.md` and `CLAUDE.md` in the `daemon-improvs` worktree.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Read `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_03.md}` and ADRs `adr-001.md` / `adr-003.md`.
- Read workflow memory files `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_03.md}`.
- Scanned existing daemon-improvement ledgers for cross-task context and read the task 01 contract ledger plus the daemon-improvs techspec ledger.
- Confirmed the target code exists in this worktree and that unrelated dirty changes live in the separate `/Users/pedronauck/dev/compozy/agh` checkout.
- Confirmed the concrete task-03 gaps were in parity proof rather than the main handler migration: current handlers already emitted canonical envelopes for most success paths, but transport tests did not yet prove canonical decoding across all required route groups or HTTP/UDS SSE equivalence.
- Removed the stray JSON tag from `core.RunStreamOverflow` so overflow state remains service-local while `internal/api/contract` owns the wire payload.
- Added canonical handler tests for daemon health envelopes, task/review run responses, stream `event`/`heartbeat`/`overflow` payloads, and request-ID-bearing error envelopes.
- Expanded `internal/api/httpapi/transport_integration_test.go` to assert canonical HTTP/UDS parity across daemon, workspaces, tasks, reviews, runs, sync, and exec routes, plus equivalent SSE frames under the same scenario.
- Added focused HTTP server-construction tests to raise `internal/api/httpapi` coverage above the task threshold.
- Verified with `go test ./internal/api/core ./internal/api/httpapi`, `go test ./internal/api/...`, `go test -cover ./internal/api/core ./internal/api/httpapi ./internal/api/contract`, and `make verify`.
- Updated workflow memory and task tracking for task `03`.
- Created local commit `1a3c7a6` (`refactor: complete shared transport contract parity`) with only the task-related code/test changes staged.

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-shared-transport-contract.md`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_03.md}`
- `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_03.md}`
- `internal/api/core/{handlers.go,interfaces.go,errors.go,sse.go,routes.go}`
- `internal/api/httpapi/{server.go,routes.go,transport_integration_test.go}`
- `internal/api/udsapi/{server.go,routes.go}`
- `internal/api/core/handlers_contract_test.go`
- Commands: `rg`, `sed`, `git status --short`, `go test ./internal/api/core ./internal/api/httpapi`, `go test ./internal/api/...`, `go test -cover ./internal/api/core ./internal/api/httpapi ./internal/api/contract`, `make verify`
