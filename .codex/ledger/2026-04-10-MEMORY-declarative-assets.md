Goal (incl. success criteria):

- Complete extensibility task 13 by wiring enabled extension skill packs and declarative provider overlays into command bootstrap, setup/preflight, and `compozy ext doctor`.
- Success requires the task deliverables, required unit/integration coverage, clean `make verify`, workflow memory/task tracking updates, and one local commit after verification.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/extensibility/task_13.md`, `_techspec.md`, `_tasks.md`, ADR-005, ADR-007, and the workflow memory files.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`.
- The approved PRD/techspec/task documents are the design baseline for this execution task; no separate brainstorming loop is needed.
- Do not touch unrelated dirty task files or use destructive git commands.
- Disabled extensions must contribute no declarative assets.
- Base bundled skills FS and base provider/agent registries must remain immutable; overlays are command-scoped.

Key decisions:

- Use one command-scoped declarative-asset bootstrap in CLI to discover enabled extensions once, activate provider/agent overlays for the command lifetime, and pass the same discovered skill-pack inventory into skills preflight.
- Treat extension-declared review providers as overlay entries that can wrap or alias base providers without mutating the base provider registry.
- Add extension skill-pack install/verify helpers in `internal/setup` that reuse the existing path/install/verify helpers against disk-backed skill directories.

State:

- Completed after implementation, focused coverage, fresh `make verify`, and task/workflow memory updates.

Done:

- Read repository instructions, required skill files, workflow memory, task 13, `_techspec.md`, `_tasks.md`, and ADR-005/007.
- Scanned relevant prior ledgers for discovery, bootstrap, runtime, and CLI handoff context.
- Captured the pre-change signal: bundled-only skills preflight, placeholder `ext doctor` drift info, fixed review-provider registry usage, and static ACP runtime spec resolution.
- Added extension skill-pack install/verify helpers plus discovery metadata needed to materialize extension skill packs from enabled extensions.
- Added command-scoped review-provider and ACP runtime overlays, wired bootstrap into `start`, `fix-reviews`, `exec`, and `fetch-reviews`, and routed review lookups through overlay-aware registry resolution.
- Extended `skills_preflight` to combine bundled skills with extension skill packs, updated doctor to report real drift/conflicts, and added unit/integration tests for install/verify, overlays, bootstrap, and doctor behavior.
- Recorded focused file coverage for task-13 files at or above 80% and reran `make verify` successfully.

Now:

- No code work remains for task 13.

Next:

- Create the required local commit and hand off the verified result.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-declarative-assets.md`
- `.compozy/tasks/extensibility/{task_13.md,_techspec.md,_tasks.md}`
- `.compozy/tasks/extensibility/adrs/{adr-005.md,adr-007.md}`
- `.compozy/tasks/extensibility/memory/{MEMORY.md,task_13.md}`
- `internal/setup/{bundle.go,install.go,verify.go,extensions.go}`
- `internal/cli/{skills_preflight.go,run.go,state.go}`
- `internal/cli/extension/doctor.go`
- `internal/core/{fetch.go}`
- `internal/core/provider/{registry.go,overlay.go}`
- `internal/core/agent/{registry_launch.go,registry_specs.go,registry_overlay.go,client.go,registry_validate.go}`
- `internal/core/run/executor/review_hooks.go`
- Commands: `rg`, `sed`, targeted `go test`, `make verify`
