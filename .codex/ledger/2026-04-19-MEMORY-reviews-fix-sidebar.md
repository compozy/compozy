Goal (incl. success criteria):

- Root-cause why `compozy reviews fix` no longer shows correct batched jobs/issues in the ACP cockpit sidebar after daemon migration.
- Success means: identify the precise regression against pre-daemon behavior, restore the correct Bubble Tea integration for daemon-backed review runs, add regression coverage, and finish with fresh `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills for this task: `systematic-debugging`, `no-workarounds`, `bubbletea`, `golang-pro`; add `testing-anti-patterns` when editing tests and `cy-final-verify` before completion.
- Do not use destructive git commands or touch unrelated dirty files.
- Treat the screenshot as a symptom only; confirm behavior from code, tests, and historical comparison.

Key decisions:

- Investigate pre-daemon vs daemon-backed run/TUI integration before proposing any fix.
- Use Bubble Tea reference guidance plus current repository patterns; do not patch the sidebar blindly.

State:

- Completed and verified with fresh `make verify`.

Done:

- Read the required skill instructions from the prompt and local skill files.
- Scanned existing ledgers for related daemon/TUI work.
- Identified historical context in ledgers for remote attach, placeholder hydration, and TUI realtime updates after daemon migration.
- Collected the daemon migration commit reference `ab0d26ca8e36`.
- Compared pre-daemon run UI bootstrap against the daemon-backed attach path and confirmed the sidebar used to be seeded directly from prepared jobs.
- Traced the daemon path to `RunManager.Snapshot()` and confirmed queued-job metadata only exists there if `job.queued` events are persisted before execution advances.
- Added a production fix in `internal/daemon/run_manager.go` to emit synthetic `job.queued` events for all prepared jobs immediately after prepare succeeds.
- Added regression coverage in `internal/daemon/run_manager_test.go` to assert daemon snapshots expose both queued review batches with correct file and issue counts.
- Ran focused tests:
  - `go test ./internal/daemon ./internal/core/run/ui -run 'Test(RunManagerSnapshotBootstrapsPreparedQueuedReviewJobs|RemoteSnapshotBootstrapHydratesUIStateBeforeLiveEvents)$' -count=1`
  - `go test ./internal/daemon -run 'TestRunManagerSnapshotIncludesJobsTranscriptAndNextCursor$' -count=1`
- Adjusted `internal/cli/root_command_execution_test.go` so the raw daemon workflow stream test validates the canonical event set after queued-job bootstrapping instead of assuming `run.started` must be the first event.
- Ran `make verify` successfully.

Now:

- Task complete; ready to summarize root cause, fix, and verification.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- Whether any additional daemon-backed batched flows besides `reviews fix` relied on the same missing queued-job snapshot contract. The fix is generic because it emits queued metadata for every prepared workflow job, not just review runs.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-reviews-fix-sidebar.md`
- `internal/daemon/{run_manager.go,run_manager_test.go,run_snapshot.go}`
- `internal/cli/root_command_execution_test.go`
- `internal/core/run/ui/{model.go,remote.go,sidebar.go}`
- `git show ab0d26ca8e36^:internal/core/run/ui/model.go`
- `make verify`
