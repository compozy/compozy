# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Complete task 07 by shipping durable snapshot integrity, richer daemon observability, bounded canonical transcript replay, parity coverage, and a clean `make verify`.

## Important Decisions

- Persist sticky run integrity in `run.db` via a singleton `run_integrity` row and merge later audit reasons into that durable state instead of short-circuiting once `Incomplete` is already true.
- Bound cold snapshot transcript payloads in the daemon read model by both message count (`200`) and approximate byte budget (`64 KiB`) so completed-run inspection stays stable for clients and CLI readers.
- Accumulate daemon journal-drop metrics per run using deltas from the latest observed journal totals so repeated snapshot reads do not inflate `daemon_journal_submit_drops_total`.

## Learnings

- The branch’s observability scaffolding already exposed the new contract types, but the service tests, transport parity fixtures, and public run-reader coverage were still mostly pinned to the older snapshot/metrics shape.
- `make verify` surfaced unrelated but real race bugs in the current worktree: the in-process CLI daemon harness closed `global.db` before the manager drained, and the managed-daemon logging integration test read `bytes.Buffer` concurrently with exec’s copy goroutine.

## Files / Surfaces

- `internal/daemon/{host.go,run_integrity.go,run_integrity_test.go,run_manager.go,service.go,service_test.go,shutdown.go,boot_integration_test.go}`
- `internal/store/{globaldb/global_db.go,globaldb/close_test.go,rundb/migrations.go,rundb/run_db.go,rundb/run_db_test.go}`
- `internal/api/{contract/{types.go,compatibility.go,contract_test.go,contract_integration_test.go},client/client_contract_test.go,httpapi/transport_integration_test.go}`
- `pkg/compozy/runs/{run.go,remote_watch.go,run_test.go}`
- `internal/cli/daemon_exec_test_helpers_test.go`

## Errors / Corrections

- Corrected a production gap where `loadRunIntegrity` stopped merging new audit reasons after the first incomplete state was persisted.
- Corrected daemon/CLI test teardown races exposed by the repository-wide `-race` gate before completion.

## Ready for Next Run

- Verification complete: targeted `go test` suites passed, targeted `go test -race` on `internal/cli`, `internal/daemon`, and `internal/store/globaldb` passed, and `make verify` passed cleanly after the race fixes.
