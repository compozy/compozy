# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Migrate shared daemon transport behavior onto canonical `internal/api/contract` payloads and parity assertions without changing the existing transport-facing service split.
- Success criteria achieved through canonical handler tests, HTTP/UDS parity tests across the route groups in scope, and clean `make verify`.

## Important Decisions

- Kept scope tight to transport ownership and parity proof because the current worktree already had most handler envelope migration from task `01`.
- Treated `RunStreamOverflow` as service-local state and removed its JSON tag so only `internal/api/contract` owns the wire format.
- Corrected earlier workflow-memory assumptions: the current worktree does not expose the shared runtime-harness package/symbols cited for task `02`, so task `03` kept parity coverage in `internal/api/httpapi/transport_integration_test.go`.

## Learnings

- HTTP/UDS request-ID parity is testable by seeding the same `X-Request-Id` header on both requests; both transports echo it through middleware and canonical error envelopes.
- SSE frame collectors must drain scanner lines before reading the terminal scanner error, or the final frame can be truncated and falsely look like a transport bug.
- The route-group parity gaps were in tests, not handler code: daemon/workspaces/runs already had some parity coverage, while tasks/reviews/sync/exec and canonical SSE equivalence were still missing.

## Files / Surfaces

- `internal/api/core/interfaces.go`
- `internal/api/core/handlers_contract_test.go`
- `internal/api/httpapi/transport_integration_test.go`

## Errors / Corrections

- Initial SSE frame collectors in the new tests could read the scanner completion signal before draining the final frame lines; fixed both collectors to read scanner errors only after the line channel closes.
- `internal/api/httpapi` coverage was initially `78.2%`; added focused server-construction tests for injected logger/engine, invalid host rejection, and cancelled-context start to raise coverage to `85.2%`.

## Ready for Next Run

- Verified commands:
  - `go test ./internal/api/core ./internal/api/httpapi`
  - `go test ./internal/api/...`
  - `go test -cover ./internal/api/core ./internal/api/httpapi ./internal/api/contract`
  - `make verify`
- Coverage after the focused contract pass:
  - `internal/api/core`: `86.3%`
  - `internal/api/httpapi`: `85.2%`
  - `internal/api/contract`: `90.6%`
- Remaining close-out work after this update: task tracking files and the scoped local commit.
- Task tracking was updated locally after verification, and the scoped code commit was created as `1a3c7a6` (`refactor: complete shared transport contract parity`).
- Tracking and memory artifacts were intentionally left out of the automatic commit per workspace policy.
