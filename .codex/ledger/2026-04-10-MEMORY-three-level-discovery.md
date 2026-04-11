Goal (incl. success criteria):

- Complete extensibility task 03 by adding three-level extension discovery (bundled/user/workspace), precedence resolution, enablement-aware filtering, declarative provider/skill-pack inventories, and tests with clean `make verify`.

Constraints/Assumptions:

- Follow repository `AGENTS.md` and `CLAUDE.md`, the task-03 spec, `.compozy/tasks/extensibility/_techspec.md`, `.compozy/tasks/extensibility/_tasks.md`, and ADRs under `.compozy/tasks/extensibility/adrs/`.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`; `brainstorming` design gate is treated as already satisfied because this is an approved PRD/techspec execution task.
- Keep scope tight to task 03. Do not touch unrelated dirty worktree entries.
- Final completion requires `go test ./internal/core/extension -cover`, `make verify`, workflow memory updates, task tracking updates, and one local commit.

Key decisions:

- Reuse the existing task-02 `Manifest`, `Ref`, and `EnablementStore` types instead of introducing parallel manifest or state models.
- Discovery must expose both “all discovered” and “enabled only” views because task 03 feeds CLI listing and runtime bootstrap differently.
- Provider and skill-pack inventories should be derived from the effective discovered entries and carry resolved absolute paths for later bootstrap/install tasks.

State:

- In progress after requirements review and implementation planning.

Done:

- Read repository instructions, required skill files, workflow memory, task-03 spec, `_techspec.md`, `_tasks.md`, and ADRs for extensibility.
- Scanned existing session ledgers for cross-agent awareness, including the completed task-02 ledger for the current extension package baseline.
- Inspected current `internal/core/extension` manifest loading/validation and enablement APIs, `skills/embed.go`, workspace root discovery, and downstream task-12/task-13 expectations.
- Confirmed no blocking contradictions across the task spec, techspec, and ADR-007.

Now:

- Implement discovery plumbing and asset extraction in `internal/core/extension`, then add the required tests.

Next:

- Run package coverage and `make verify`, update workflow memory and task tracking, then create the required local commit if verification is clean.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-three-level-discovery.md`
- `.compozy/tasks/extensibility/{task_03.md,_techspec.md,_tasks.md}`
- `.compozy/tasks/extensibility/adrs/adr-007.md`
- `.compozy/tasks/extensibility/task_12.md`
- `.compozy/tasks/extensibility/task_13.md`
- `.compozy/tasks/extensibility/memory/{MEMORY.md,task_03.md}`
- `internal/core/extension/{doc.go,manifest.go,manifest_load.go,enablement.go}`
- `internal/core/workspace/config.go`
- `skills/embed.go`
- Commands: `rg`, `sed`, `go test ./internal/core/extension -cover`, `make verify`
