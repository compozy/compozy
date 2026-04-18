Goal (incl. success criteria):

- Add the missing QA planning and QA execution tail tasks to `.compozy/tasks/daemon`, matching the established repo pattern used in `tasks-ui` but tailored to the daemon feature.
- Success means: `_tasks.md`, `_meta.md`, and new `task_17.md` / `task_18.md` exist with valid structure, coherent dependencies, task-memory stubs, and task validation passes.

Constraints/Assumptions:

- Follow `cy-create-tasks`, `qa-report`, and `qa-execution` skill requirements.
- Do not touch unrelated git changes or use destructive git commands.
- `.compozy/config.toml` is absent, so built-in task types apply.
- This is task-catalog maintenance, not product-code implementation.

Key decisions:

- Skip `brainstorming` because this is not a net-new feature design; it is a targeted backlog alignment against an existing approved pattern.
- Add two daemon-specific tail tasks: a `docs` QA planning task and a `test` QA execution task.
- Root both tasks at `.compozy/tasks/daemon/qa/`.
- Treat browser/web validation as explicitly blocked or out of scope unless a real daemon web surface exists on the future execution branch.

State:

- Ready to hand off with partial verification only.

Done:

- Read repo instructions and required skill files.
- Scanned existing session ledgers for daemon/task QA context.
- Compared `.compozy/tasks/daemon` with the reference tail in `/Users/pedronauck/Dev/compozy/_worktrees/tasks-ui/.compozy/tasks/tasks-ui`.
- Read daemon TechSpec testing/risk sections and relevant ADRs.
- Added `task_17` and `task_18`, updated `_tasks.md`, `_meta.md`, workflow memory, and task-memory stubs.
- Ran `go run ./cmd/compozy validate-tasks --name daemon` successfully: `all tasks valid (18 scanned)`.
- Ran `make verify`; it failed during lint/typecheck on unrelated WIP under `internal/store/rundb/` because `sequenceValue` is undefined in `run_db.go`.

Now:

- Prepare the final handoff with accurate verification evidence and the unrelated blocker called out explicitly.

Next:

- If needed later, coordinate with the owner of `internal/store/rundb/` or expand scope to repair that branch-wide verification blocker.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether the user wants this task to stop at task-catalog changes or also absorb the unrelated `internal/store/rundb` verification failure.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-daemon-qa-tasks.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/_meta.md`
- `.compozy/tasks/daemon/task_17.md`
- `.compozy/tasks/daemon/task_18.md`
- `.compozy/tasks/daemon/memory/task_17.md`
- `.compozy/tasks/daemon/memory/task_18.md`
- `.compozy/tasks/daemon/memory/MEMORY.md`
- `.compozy/tasks/daemon/_techspec.md`
- `/Users/pedronauck/Dev/compozy/_worktrees/tasks-ui/.compozy/tasks/tasks-ui/task_18.md`
- `/Users/pedronauck/Dev/compozy/_worktrees/tasks-ui/.compozy/tasks/tasks-ui/task_19.md`
