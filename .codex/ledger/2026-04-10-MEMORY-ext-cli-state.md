Goal (incl. success criteria):

- Implement extensibility task 12 by adding the builtin `compozy ext` management surface (`list`, `inspect`, `install`, `uninstall`, `enable`, `disable`, `doctor`), persisting operator-local enablement state outside the repo, adding the required tests/coverage, and finishing with clean `make verify`, workflow memory/tracking updates, and one local commit.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/extensibility/task_12.md`, `.compozy/tasks/extensibility/_techspec.md`, `.compozy/tasks/extensibility/_tasks.md`, and ADR-005 / ADR-007.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`; the approved PRD/techspec/task workflow satisfies the brainstorming design gate for this implementation task.
- Keep CLI implementation within the seven-file grouping mandated by task 12.
- Reuse `internal/core/extension` discovery and enablement APIs; do not duplicate scanning logic.
- Do not touch unrelated dirty worktree files or use destructive git commands.
- Final completion requires the explicit task tests, package coverage >= 80%, fresh `make verify`, workflow memory updates, task tracking updates, self-review, and one local commit.

Key decisions:

- Treat the task spec and techspec as the approved design baseline; no separate design loop is needed before implementation.
- `list` should show raw discovered declarations plus an `active` column derived from precedence + enablement, while `inspect` / `enable` / `disable` should resolve the effective declaration name after precedence resolution.
- `install` should always record a disabled user-state file immediately after copying into `~/.compozy/extensions/<name>/`, and `uninstall` should only ever remove user-scope directories by name.
- `doctor` should validate manifests and emit placeholder info for skill/provider drift until task 13 adds the real overlay-drift checks.

State:

- Implemented and fully verified; workflow/task tracking updates and the required local commit remain.

Done:

- Read `AGENTS.md`, `CLAUDE.md`, shared workflow memory, current task memory, task 12 spec, `_techspec.md`, `_tasks.md`, ADR-005, and ADR-007.
- Scanned existing session ledgers for cross-agent awareness, including prior extensibility tasks and the extension foundation/discovery/bootstrap context.
- Confirmed no blocking contradictions across task 12, the techspec CLI management surface, and the related ADRs.
- Captured the required execution checklist and pre-change signal: there is currently no `internal/cli/extension` package and the root command does not yet register `compozy ext`.
- Added `internal/cli/extension/{root,display,install,enablement,doctor}.go` plus black-box tests in `display_test.go` / `doctor_test.go`.
- Registered `compozy ext` on the root Cobra command and documented the management surface in root help text.
- Implemented `list`, `inspect`, `install`, `uninstall`, `enable`, `disable`, and `doctor` on top of `internal/core/extension` discovery and enablement.
- Added unit and round-trip integration coverage for install → enable → list → disable → uninstall plus direct helper-path tests; `internal/cli/extension` now reports `80.7%` statement coverage.
- Ran targeted verification successfully:
  - `go test ./internal/cli/extension -cover -count=1`
  - `go test ./internal/cli -count=1`
- Ran the full repository gate successfully: `make verify` passed with `0 issues`, `DONE 1418 tests`, and a successful build.

Now:

- Update workflow/task tracking files for task 12, review the final diff, and create the required local commit.

Next:

- Optional cleanup only after commit: remove this ledger if no follow-up on task 12 is needed.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-ext-cli-state.md`
- `.compozy/tasks/extensibility/{task_12.md,_techspec.md,_tasks.md}`
- `.compozy/tasks/extensibility/adrs/{adr-005.md,adr-007.md}`
- `.compozy/tasks/extensibility/memory/{MEMORY.md,task_12.md}`
- `internal/cli/root.go`
- `internal/cli/extension/{root.go,display.go,install.go,enablement.go,doctor.go,display_test.go,doctor_test.go}`
- Commands: `go test ./internal/cli/extension -cover -count=1`, `go test ./internal/cli -count=1`, `make verify`
