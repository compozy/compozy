# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Establish `internal/api/contract` as the canonical daemon transport contract for daemon, workspace, task, review, run, sync, and exec surfaces without silently expanding into a full transport migration.

## Important Decisions

- The authoritative codebase for this task is `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`, not the unrelated `/Users/pedronauck/dev/compozy/agh` checkout.
- Use `internal/api/core/routes.go` in this worktree as the route inventory source of truth.
- Prefer contract-owned DTO/error/SSE/timeout definitions with thin `internal/api/core` aliases or wrappers to preserve current service seams and minimize churn.
- Keep session snapshot transport ownership in `internal/api/contract` too, then convert at daemon and UI boundaries so the contract package stays free of runtime-package imports.

## Learnings

- `internal/api/contract` does not exist yet in this worktree; transport DTOs, error envelope types, cursor logic, and SSE heartbeat/overflow helpers are still owned by `internal/api/core`.
- `internal/api/client` and `pkg/compozy/runs` both duplicate daemon wire shapes (`run` payloads, `next_cursor`, heartbeat/overflow payloads, and transport error decoding), so compatibility coverage matters immediately.
- Adopting `internal/api/contract` from `pkg/compozy/runs` exposed an import cycle through `transcript -> model -> journal`; moving the session snapshot wire shape into the contract package resolved it without reintroducing duplicated transport DTOs.

## Files / Surfaces

- `internal/api/contract/*`
- `internal/api/core/{interfaces.go,errors.go,sse.go,routes.go,handlers.go}`
- `internal/api/client/{client.go,runs.go}`
- `pkg/compozy/runs/{run.go,remote_watch.go}`
- `internal/daemon/run_snapshot.go`
- `internal/core/run/ui/{remote.go,remote_test.go}`
- `internal/api/httpapi/transport_integration_test.go`

## Errors / Corrections

- Initial exploration started in the wrong checkout because the prompt cwd differed from the task worktree; corrected before code changes.
- The first `make verify` pass surfaced a journal-package test import cycle caused by runtime-type imports in the contract package; corrected by making the contract own session snapshot DTOs and adding explicit conversion at daemon/UI seams.

## Ready for Next Run

- Task complete after passing `go test ./internal/api/... ./internal/daemon ./pkg/compozy/runs`, `go test ./internal/core/run/journal ./internal/core/run/ui`, `go test -tags integration ./internal/api/contract`, `go test -cover ./internal/api/contract` (`90.6%`), and `make verify`.
