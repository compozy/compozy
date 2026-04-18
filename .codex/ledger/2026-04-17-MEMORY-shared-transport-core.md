Goal (incl. success criteria):

- Implement daemon task `04` (`Shared Transport Core`) with a shared transport-neutral handler core, aligned HTTP and UDS route registration, request IDs, JSON error envelopes, and SSE cursor/heartbeat/overflow behavior.
- Success means: task `04` requirements and explicit tests are satisfied, workflow memory/tracking are updated correctly, and `make verify` passes before completion.

Constraints/Assumptions:

- Must follow the daemon task workflow using `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- Scope stays tight to task `04`; later run-manager and sync implementations remain injected behind interfaces rather than embedded into transport code.
- Do not touch unrelated dirty worktree files.
- No destructive git commands without explicit user permission.

Key decisions:

- Use the approved daemon techspec and ADR-003 resource model as the source of truth instead of inventing new command-style routes.
- Build one shared API core under `internal/api/core` and keep HTTP/UDS as thin server/wiring layers.
- Treat current daemon bootstrap (`internal/daemon`) plus `globaldb` and `rundb` as the primary local seams; missing higher-level daemon services will be expressed as injected interfaces.
- The current baseline is explicit: `internal/api` does not exist, `go.mod` does not include Gin, and no request-ID or transport-error layer exists yet.

State:

- Completed after transport implementation, coverage closure, task/memory updates, and clean verification.

Done:

- Read repository instructions from workspace `AGENTS.md` and `CLAUDE.md`.
- Read the required skills for workflow memory, task execution, final verification, Go implementation, test hygiene, and brainstorming.
- Read daemon workflow memory (`MEMORY.md`, `task_04.md`), daemon task docs (`task_04.md`, `_tasks.md`), `_techspec.md`, and ADRs `001` and `003`.
- Read relevant daemon cross-agent ledgers for architecture and task context.
- Inspected current daemon/home/store code and AGH transport reference files.
- Confirmed there are no blocking spec conflicts before implementation.
- Captured the execution checklist and pre-change signal for task `04`.
- Added `github.com/gin-gonic/gin` to the module and created the shared transport packages under `internal/api/core`, `internal/api/httpapi`, and `internal/api/udsapi`.
- Implemented request ID propagation, `TransportError`, shared route registration, SSE cursor/heartbeat/overflow helpers, localhost HTTP binding, and UDS socket permission handling.
- Added transport unit and integration coverage, including HTTP/UDS parity, health/metrics, request IDs, invalid/stale cursors, resume semantics, overflow, and terminal stream behavior.
- Extended `internal/daemon/boot.go` with the host seam used to persist the bound HTTP port.
- Raised `internal/api/core` coverage to `85.0%`.
- Ran focused transport verification and the full repository gate successfully with `make verify`.
- Updated daemon workflow memory plus `task_04.md` / `_tasks.md` to mark the task complete.

Now:

- Final response and local commit only.

Next:

- Create the required local commit with code changes only; keep tracking-only files out of the commit.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-shared-transport-core.md`
- `.compozy/tasks/daemon/memory/MEMORY.md`
- `.compozy/tasks/daemon/memory/task_04.md`
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_04.md`
- `.compozy/tasks/daemon/adrs/adr-001.md`
- `.compozy/tasks/daemon/adrs/adr-003.md`
- `go.mod`
- `go.sum`
- `internal/daemon/boot.go`
- `internal/api/core/*`
- `internal/api/httpapi/*`
- `internal/api/udsapi/*`
- Commands: `go test ./internal/api/... ./internal/daemon/...`, `go test -cover ./internal/api/core`, `make verify`
