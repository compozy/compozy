# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Harden ACP-backed runtime supervision in the current AGH architecture by persisting liveness/stall evidence, improving subprocess stop semantics, and making boot/reconcile classification honest for crashed/orphaned/stalled work.
- Keep ownership boundaries intact: `session.Manager` is the runtime seam, not a new executor stack.

## Important Decisions

- Map the task doc's stale `internal/core/...` references onto the actual repo surfaces:
  - ACP/session supervision: `internal/session`, `internal/acp`
  - Process control: `internal/subprocess`, `internal/hooks`
  - Recovery/read-side reconcile: `internal/daemon/task_runtime.go`, `internal/observe/reconcile.go`
  - Durable state: `internal/store/types.go`, `internal/store/globaldb/*`, `internal/task/*`
- Use `store.SessionMeta` plus the global `sessions` index for durable session liveness data.
- Use `task.Run.Metadata` / `task_runs.metadata_json` for run-specific recovery evidence instead of adding a new task-run store.

## Learnings

- The imported task spec is conceptually correct but its file paths are stale for this worktree; the required behavior maps cleanly onto the existing AGH packages above.
- Most ACP fault primitives already exist in `internal/testutil/acpmock` (`disconnect`, raw JSON-RPC injection, cancel blocking); the missing work is durable classification and runtime reporting, not fixture invention.
- The blocked-cancel ACP path can still surface an `error` prompt-stream event during teardown even when the persisted session stop is correctly recorded as `user_canceled`; runtime assertions need to check both stream behavior and durable stop classification.
- Immediate prompt or stop operations after harness session creation were racing session registration; `createFixtureBackedSession` now waits until the session is queryable before returning.

## Files / Surfaces

- `internal/session/{session.go,manager_helpers.go,manager_lifecycle.go,manager_start.go,resume_repair.go}`
- `internal/acp/{client.go,handlers.go,types.go}`
- `internal/subprocess/{process.go,signals_unix.go,signals_windows.go}`
- `internal/hooks/executor_subprocess.go`
- `internal/daemon/{task_runtime.go,daemon_acpmock_faults_integration_test.go,daemon_acpmock_helpers_integration_test.go}`
- `internal/observe/{observer.go,reconcile.go}`
- `internal/store/{types.go,meta.go}`
- `internal/store/globaldb/{global_db.go,global_db_session.go,global_db_task.go}`
- `internal/testutil/acpmock/*`
- `web/.storybook/preview.ts`

## Errors / Corrections

- Baseline Windows build already fails before task changes: `GOOS=windows GOARCH=amd64 go test ./internal/subprocess` reports missing forced-exit helpers from the subprocess/hook path. This is part of the task scope because the spec requires explicit Windows fallback semantics.
- Full-gate verification exposed separate repo-gate drift outside the backend task scope: Storybook still called the removed `useSessionStore().clearSession()` API and now uses `clearAllDrafts()` so `tsgo --noEmit` can pass under `make verify`.

## Ready for Next Run

- Task completed.
- Durable liveness now flows through `store.SessionLivenessMeta`, the global `sessions` index, `session.ClassifyInactiveMetaForRecovery`, and task-run recovery metadata.
- Verification evidence: targeted ACP fault tests, focused Go package tests, Windows cross-build checks, and a clean `make verify`.
