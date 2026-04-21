Goal (incl. success criteria):

- Implement task `task_01.md` (`Canonical Daemon Contract Foundation`) in the `daemon-improvs` worktree.
- Success means `internal/api/contract` becomes the owned source for daemon transport DTOs, error envelope semantics, SSE/cursor helpers, timeout classes, route inventory metadata, and explicit run compatibility notes, with required tests and clean `make verify`.

Constraints/Assumptions:

- Work only in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`.
- Must follow `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- Must read and update `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_01.md}`.
- Must keep scope tight to contract extraction and freezing; do not silently migrate unrelated runtime behavior.
- No destructive git commands without explicit user permission.

Key decisions:

- The task belongs to the `daemon-improvs` worktree, not `/Users/pedronauck/dev/compozy/agh`; that original checkout is a different repo state and not the source of truth for this task.
- Use `internal/api/core/routes.go` in `daemon-improvs` as the authoritative current route inventory.
- Treat the task as a contract extraction/freeze: define the canonical contract now and keep handler/client adoption as minimal as needed for compatibility and tests.

State:

- Completed after clean focused verification, contract integration coverage, and full `make verify`.

Done:

- Read root `AGENTS.md` and `CLAUDE.md` in the `daemon-improvs` worktree.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Read `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_01.md}` and ADRs `adr-001.md` / `adr-004.md`.
- Read workflow memory templates at `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_01.md}`.
- Scanned relevant existing ledgers for daemon-improvements context and prior transport work.
- Confirmed the target code exists in this worktree: `internal/api/core/{interfaces.go,errors.go,sse.go,routes.go}`, `internal/api/client/{client.go,runs.go,reviews_exec.go}`, and `pkg/compozy/runs/run.go`.
- Added `internal/api/contract` with canonical daemon DTOs, transport errors, SSE/cursor helpers, route inventory metadata, timeout taxonomy, and compatibility notes.
- Redirected JSON-facing ownership in `internal/api/core`, `internal/api/client`, and `pkg/compozy/runs` to the contract package while keeping service interfaces transport-neutral.
- Added contract unit coverage for route inventory parity, timeout policies, JSON round-trips, cursor semantics, error envelopes, heartbeat/overflow payloads, and compatibility notes.
- Added integration coverage for daemon health, run snapshot decode, and run stream heartbeat/overflow decode through current handlers.
- Fixed the `pkg/compozy/runs` transport fixtures for the canonical flat transport error envelope.
- Resolved a runtime import-cycle regression by moving session snapshot DTO ownership into the contract package and converting at daemon/UI seams.
- Verified with `go test ./internal/api/... ./internal/daemon ./pkg/compozy/runs`, `go test ./internal/core/run/journal ./internal/core/run/ui`, `go test -tags integration ./internal/api/contract`, `go test -cover ./internal/api/contract` (`90.6%`), and `make verify`.

Now:

- Update workflow memory and task tracking, then create the local code commit.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-canonical-daemon-contract.md`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_01.md}`
- `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_01.md}`
- `internal/api/core/{interfaces.go,errors.go,sse.go,routes.go,handlers.go}`
- `internal/api/contract/*`
- `internal/api/client/{client.go,runs.go,reviews_exec.go}`
- `pkg/compozy/runs/run.go`
- `internal/daemon/run_snapshot.go`
- `internal/core/run/ui/{remote.go,remote_test.go}`
- Commands: `go test ./internal/api/... ./internal/daemon ./pkg/compozy/runs`, `go test ./internal/core/run/journal ./internal/core/run/ui`, `go test -tags integration ./internal/api/contract`, `go test -cover ./internal/api/contract`, `make verify`
