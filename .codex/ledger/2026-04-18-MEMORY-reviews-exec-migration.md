Goal (incl. success criteria):

- Complete daemon task `15` (`Reviews and Exec Flow Migration`) end to end.
- Success means review fetch/list/show/fix and ad hoc exec flows run through the daemon control plane, review Markdown artifacts remain authoritative in the workspace, exec input/output behavior stays compatible, required focused tests pass, and `make verify` passes.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/task_15.md`, `_techspec.md`, `_tasks.md`, ADR-002/003/004, and workflow memory files.
- Required skills for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`. `brainstorming` was read because behavior changes, but the approved task/techspec are the design baseline for this implementation.
- Worktree is already dirty in unrelated task-tracking and ledger files. Do not revert or modify unrelated files.
- No destructive git commands without explicit user permission. Completion requires fresh `make verify`.

Key decisions:

- Reuse `internal/daemon.RunManager` plus existing review/exec parsers rather than inventing new execution paths.
- Keep top-level `fetch-reviews` and `fix-reviews` working for compatibility while adding daemon-backed `reviews` subcommands as the canonical family.
- Preserve existing CLI prompt-source validation and JSON/raw-JSON error behavior for `exec`; only lifecycle ownership moves to daemon APIs.

State:

- Completed after focused coverage expansion, full `make verify`, and task/memory updates.

Done:

- Read workspace instructions, required skill docs, workflow memory, task docs, techspec, ADRs, and relevant daemon ledgers from tasks 05/10/11/14.
- Reconciled current worktree state with `git status --short`.
- Verified the daemon-backed review/exec surfaces that were already present in the worktree and closed the remaining task-15 gap by adding focused coverage:
  - `internal/cli/reviews_exec_daemon_additional_test.go`
  - `internal/daemon/review_exec_transport_service_test.go`
  - `internal/api/client/reviews_exec_test.go`
- Hardened `internal/cli/agents_commands_test.go` so unknown-agent validation no longer inherits ambient daemon-home env during parallel package runs.
- Refreshed coverprofiles and confirmed migration-seam coverage above the task target:
  - `internal/cli/reviews_exec_daemon.go` 81.4%
  - `internal/daemon/review_exec_transport_service.go` 83.5%
  - `internal/api/client/reviews_exec.go` 80.7%
- Ran focused package validation plus `make verify` successfully.
- Updated workflow memory, task memory, and task tracking for task 15 completion.

Now:

- Prepare the required local commit, excluding tracking-only workflow files.

Next:

- None after the commit is created.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-reviews-exec-migration.md`
- `.compozy/tasks/daemon/{task_15.md,_tasks.md,_techspec.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_15.md}`
- `.compozy/tasks/daemon/adrs/{adr-002.md,adr-003.md,adr-004.md}`
- `internal/{daemon,api/client,cli}`
- `internal/core/{fetch.go,run/exec,run/preflight,reviews,extension}`
- `internal/store/globaldb`
- Commands: `rg`, `sed`, `git status --short`
