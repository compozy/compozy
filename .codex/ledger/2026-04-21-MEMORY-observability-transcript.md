Goal (incl. success criteria):

- Implement task `task_07.md` (`Observability, Snapshot Integrity, and Transcript Assembly`) in the `daemon-improvs` worktree.
- Success means daemon health/metrics expose the richer contract from the TechSpec, `run.db` persists sticky integrity state for `RunSnapshot.Incomplete`, canonical transcript replay works for cold readers, required unit/integration coverage lands, and `make verify` passes cleanly.

Constraints/Assumptions:

- Work only in `/Users/pedronauck/Dev/compozy/_worktrees/daemon-improvs`.
- Must follow `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- Must read and update `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_07.md}`.
- Keep scope tight to observability, snapshot integrity, transcript assembly, and parity/read-model consumers. Record follow-up gaps instead of expanding silently.
- No destructive git commands without explicit user permission.
- The worktree already contains unrelated in-progress task-05 changes; integrate carefully and do not revert them.

Key decisions:

- The `daemon-improvs` worktree is the source of truth for this task; the separate `/Users/pedronauck/dev/compozy/agh` checkout is unrelated dirty state.
- Treat `_tasks.md` showing task `05` as pending as stale tracking, because `task_05.md` itself is already marked `completed`.
- Use the TechSpec sections `Snapshot Integrity Semantics`, `API Endpoints`, `Monitoring and Observability`, `Build Order`, and `Key Decisions`, plus ADR-001/003/004, as the implementation boundary.

State:

- Completed after observability/integrity/transcript implementation, race fixes, and clean repository-wide verification.

Done:

- Read root `AGENTS.md` and `CLAUDE.md` in the `daemon-improvs` worktree.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Read `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_07.md}` and ADRs `adr-001.md`, `adr-003.md`, and `adr-004.md`.
- Read workflow memory files `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_07.md}`.
- Scanned related daemon-improvement ledgers for contract and runtime-hardening context.
- Confirmed the task must be implemented in the `daemon-improvs` worktree because the referenced source paths only exist there.
- Extended daemon health/metrics to the richer contract, including uptime, database diagnostics, active-run-by-mode, reconcile diagnostics, integrity counts, and full Prometheus metric families.
- Added durable `run_integrity` persistence and snapshot assembly logic for sticky incomplete reasons plus bounded cold transcript replay.
- Added regression and parity coverage across `internal/daemon`, `internal/store/rundb`, `internal/api/{contract,client,httpapi}`, and `pkg/compozy/runs`.
- Fixed repository-wide verification blockers exposed by the worktree: helper-function lint limits, `globaldb` close-vs-update teardown races, and concurrent stderr buffer reads in managed-daemon logging integration tests.
- Passed targeted `go test` suites, targeted `go test -race ./internal/cli ./internal/daemon ./internal/store/globaldb`, and `make verify`.

Now:

- Update task/workflow tracking and create the scoped local commit, keeping ledger/memory artifacts out of the commit.

Next:

- None after tracking updates and the local commit.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-observability-transcript.md`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_07.md}`
- `.compozy/tasks/daemon-improvs/memory/{MEMORY.md,task_07.md}`
- `internal/daemon/{service.go,run_snapshot.go,transport_mappers.go}`
- `internal/store/rundb/{run_db.go,migrations.go}`
- `internal/core/run/transcript/{model.go,render.go}`
- `internal/api/client/runs.go`
- `pkg/compozy/runs/{replay.go,watch.go}`
- Commands: `rg`, `sed`, `git status --short`, targeted `go test`, `make verify`
