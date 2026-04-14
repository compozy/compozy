Goal (incl. success criteria):

- Fix two related ACP cockpit/runtime issues:
- `fix-reviews` must preserve deterministic batch ordering when effective concurrency is `1`.
- Workflow runs must become terminal as soon as all jobs finish, even if the cockpit stays open until the operator presses `q`.
- Success requires root-cause code changes, regression coverage, and a clean `make verify`.

Constraints/Assumptions:

- Follow repository `AGENTS.md` and `CLAUDE.md`; do not touch unrelated dirty files.
- Accepted product decision: official run status changes to terminal on job completion, not on cockpit exit.
- Required skills in effect for this task: `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `cy-final-verify`.

Key decisions:

- Treat the `fix-reviews` ordering bug and delayed run completion as separate root causes sharing the same cockpit surface symptom.
- Preserve current drain/force shutdown semantics while changing only normal completion timing.
- Do not add a new public post-run status.

State:

- Completed with clean verification.

Done:

- Reproduced and traced the issue through real run artifacts under `/Users/pedronauck/Dev/compozy/_worktrees/core-tasks/.compozy/runs`.
- Confirmed `fix-reviews` batch order inversion: `batch_002` started and finished before `batch_001`.
- Confirmed `start` sequential task execution is correct, but `run.completed` is delayed until long after the final `job.completed`.
- Identified executor seams: `internal/core/run/executor/shutdown.go`, `internal/core/run/executor/execution.go`, `internal/core/run/executor/runner.go`, and `pkg/compozy/runs/status.go`.
- Persisted the accepted implementation plan in `.codex/plans/2026-04-14-batch-run-status.md`.
- Patched the executor so normal completion finalizes run artifacts before blocking on cockpit exit.
- Patched worker dispatch so effective serial execution preserves prepared batch order.
- Added regression coverage for normal-completion finalization timing, serial batch ordering, and run summary derivation from early `result.json`.
- Targeted verification passed: `go test ./internal/core/run/executor ./pkg/compozy/runs -count=1`.
- Full verification passed: `make verify`.

Now:

- Prepare final handoff with verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-14-batch-run-status.md`
- `.codex/ledger/2026-04-14-MEMORY-batch-run-status.md`
- `internal/core/run/executor/execution.go`
- `internal/core/run/executor/shutdown.go`
- `internal/core/run/executor/execution_acp_integration_test.go`
- `internal/core/run/executor/execution_test.go`
- `pkg/compozy/runs/status.go`
- `pkg/compozy/runs/run_test.go`
- Verification commands: `go test ...`, `make verify`
