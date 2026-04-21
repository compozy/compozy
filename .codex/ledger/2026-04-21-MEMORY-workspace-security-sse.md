Goal (incl. success criteria):

- Implement `daemon-web-ui` task `07` (`Active Workspace, Browser Security, and SSE Compatibility`) in the `daemon-web-ui` worktree.
- Success means: browser HTTP requests use explicit active-workspace semantics via `X-Compozy-Workspace-ID`, browser-only Host/Origin/CSRF hardening is enforced on HTTP without regressing UDS/CLI flows, run SSE semantics match the TechSpec and remain compatible with existing clients/readers, required tests pass at >=80% on touched packages, workflow memory/tracking are updated correctly, and `make verify` passes before completion/commit.

Constraints/Assumptions:

- Execute in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-web-ui`, not the unrelated `agh` checkout.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- Keep scope tight to task `07`; record follow-up work instead of silently expanding into later static-serving or frontend tasks.
- Worktree is already dirty in unrelated task docs / changelog / workflow files; do not revert or modify unrelated edits.
- No destructive git commands without explicit user permission.

Key decisions:

- Use the task docs, `daemon-web-ui` TechSpec sections `Active Workspace Model`, `Streaming Contract`, `Security`, and `Known Risks`, plus ADR-002/003 as the source of truth.
- Keep browser-specific trust rules in HTTP middleware/wiring; preserve shared handler compatibility and do not leak browser policy into UDS.
- Preserve existing non-browser query/body workspace flows while layering browser header semantics on the browser-facing HTTP path.

State:

- In progress after implementation and focused verification; final gate, tracking updates, and commit remain.

Done:

- Read repository instructions from the worktree `AGENTS.md` and `CLAUDE.md`.
- Read required skill docs for workflow memory, task execution, final verification, Go implementation, and test hygiene.
- Read workflow memory files `MEMORY.md` and `task_07.md`.
- Read `task_06.md`, `task_07.md`, `_tasks.md`, TechSpec sections for active workspace / streaming / security / testing, and ADR-002 / ADR-003.
- Scanned relevant cross-agent ledgers, especially `shared-transport-core`, `daemon-web-ui`, `active-run-watchers`, and `daemon-query-layer`.
- Confirmed the dedicated `daemon-web-ui` worktree matches the task spec and the unrelated `agh` checkout does not.
- Captured the pre-change signal:
  - browser workspace handling still relies on `workspace` query/body values and the OpenAPI contract test currently forbids `X-Compozy-Workspace-ID`
  - HTTP server has no Host/Origin/CSRF middleware
  - SSE helpers still emit `heartbeat` / `overflow` instead of `run.heartbeat` / `run.overflow`
- Implemented shared active-workspace context helpers and updated handlers to prefer request-context workspace IDs while preserving legacy query/body fallbacks.
- Added browser-only HTTP middleware for Host/Origin validation, active-workspace header validation, and double-submit CSRF without changing UDS behavior.
- Updated the run SSE contract to use canonical `run.snapshot`, `run.event`, `run.heartbeat`, and `run.overflow` events with cursor/query resume compatibility.
- Added compatibility coverage for `internal/api/client` and `pkg/compozy/runs`, plus HTTP/OpenAPI/security coverage and helper tests that raised `internal/api/httpapi` coverage above the task target.
- Corrected `/api/sync` so browser-header requests resolve workspace context properly and missing browser workspace now reports `workspace_context_missing` (`412`) instead of the old validation path.
- Updated workflow memory (`MEMORY.md`, `task_07.md`) with the durable contract shift to header-based browser workspace semantics.

Now:

- Run the full repository verification gate, then update task tracking and create the required local commit if clean.

Next:

- Update `task_07.md` and `_tasks.md` only after `make verify` passes and a self-review finds no remaining regressions.
- Create one local commit for the task changes without touching unrelated dirty files.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether the browser mutation CSRF token should be enforced on all POST/PATCH/DELETE browser API routes or only the browser-scoped task/review/run surfaces referenced by the TechSpec.
  - Resolved locally by enforcing CSRF on browser-like mutating HTTP requests only; UDS requests bypass the middleware because it is not installed there.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-workspace-security-sse.md`
- `.compozy/tasks/daemon-web-ui/{_techspec.md,_tasks.md,task_06.md,task_07.md}`
- `.compozy/tasks/daemon-web-ui/memory/{MEMORY.md,task_07.md}`
- `.compozy/tasks/daemon-web-ui/adrs/{adr-002.md,adr-003.md}`
- `internal/api/core/{middleware.go,handlers.go,handlers_error_paths_test.go,handlers_test.go,handlers_service_errors_test.go,sse.go,sse_test.go}`
- `internal/api/httpapi/{browser_middleware.go,browser_middleware_test.go,server.go,openapi_contract_test.go,transport_integration_test.go}`
- `internal/api/client/{runs.go,runs_test.go}`
- `pkg/compozy/runs/{remote_watch.go,run.go,transport_test.go}`
- `openapi/compozy-daemon.json`
- Commands: `gofmt -w ...`, targeted `go test ...`, `go test -cover ...`, pending `make verify`
