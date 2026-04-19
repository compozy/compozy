Goal (incl. success criteria):

- Align the public CLI surface with `.compozy/tasks/daemon/_techspec.md` by removing legacy top-level `validate-tasks`, `fetch-reviews`, and `fix-reviews` commands in favor of `compozy tasks validate` and `compozy reviews fetch|fix`.
- Success means root help, command registration, tests, and user-facing docs no longer advertise the legacy commands, canonical subcommands work correctly, and `make verify` passes.

Constraints/Assumptions:

- Follow repo `AGENTS.md` / `CLAUDE.md` instructions, including required skills, no destructive git commands, and completion only after fresh `make verify`.
- User explicitly requested `systematic-debugging`, `no-workarounds`, and `golang-pro`; `testing-anti-patterns` is also required because tests/docs will change.
- Do not modify unrelated dirty files; current unrelated worktree content is only `.codex/ledger/2026-04-18-MEMORY-smux-compozy-pairing.md`.
- Treat the daemon techspec as the source of truth over older task memory that preserved compatibility aliases.

Key decisions:

- Root cause is a compatibility-driven implementation choice that kept legacy top-level commands registered and documented even though the daemon techspec canonizes `tasks` and `reviews` command families and rejects deprecated aliases by default.
- Fix by removing the legacy command registrations from the public root surface and moving task validation onto `tasks validate`, rather than hiding help text or adding warning shims.

State:

- Completed with focused validation and full `make verify` passing.

Done:

- Read relevant workspace instructions and required skill files.
- Read cross-agent daemon ledgers relevant to the command-surface migration, especially `2026-04-18-MEMORY-daemon-command-surface.md`, `2026-04-18-MEMORY-reviews-exec-migration.md`, and `2026-04-11-MEMORY-fetch-reviews-refactor.md`.
- Reproduced the current CLI behavior with `go run ./cmd/compozy --help`, `go run ./cmd/compozy tasks --help`, and `go run ./cmd/compozy reviews --help`.
- Confirmed the mismatch:
  - root help still lists `validate-tasks`, `fetch-reviews`, and `fix-reviews`
  - `tasks` only exposes `run`
  - `reviews` already exposes `fetch|list|show|fix`
- Located the main implementation sites in `internal/cli/root.go`, `internal/cli/commands.go`, `internal/cli/validate_tasks.go`, `internal/cli/reviews_exec_daemon.go`, tests under `internal/cli`, and user docs in `README.md` / `skills/compozy/**`.
- Refactored the public CLI surface:
  - removed root registration/help for `validate-tasks`, `fetch-reviews`, and `fix-reviews`
  - added `tasks validate`
  - kept `reviews fetch|list|show|fix` as the review surface
- Updated canonical help text and review-fetch validation errors so user-facing output now points to `tasks validate` / `reviews fetch`.
- Updated README, Compozy skill docs/references, extension docs, and the daemon QA test case to use the canonical command families instead of the legacy top-level commands.
- Updated impacted CLI/docs tests and removed the direct test dependencies on the legacy top-level review command constructors.
- Ran focused validation successfully:
  - `go test ./internal/cli ./internal/core ./test -count=1`
- Ran the full repository gate successfully:
  - `make verify`
  - Result: `0 issues`, `DONE 2384 tests, 1 skipped`, successful `go build ./cmd/compozy`

Now:

- None.

Next:

- Report the final verified CLI surface change and note any residual non-user-facing legacy references still outside the public command surface.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether any non-user-facing internal strings should also be renamed from `fix-reviews` / `fetch-reviews` to canonical command paths, or whether that should stay as internal execution labels only.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-cli-command-cleanup.md`
- `.compozy/tasks/daemon/_techspec.md`
- `internal/cli/{root.go,commands.go,validate_tasks.go,reviews_exec_daemon.go,state.go,workspace_config.go,form.go}`
- `internal/cli/{root_test.go,root_command_execution_test.go,validate_tasks_test.go,migrate_command_test.go,form_test.go,commands_test.go,workspace_config_test.go,extensions_bootstrap_test.go,reviews_exec_daemon_additional_test.go}`
- `internal/core/extension/{manager_constants.go,manager_test.go}`
- `README.md`
- `skills/compozy/{SKILL.md,references/cli-reference.md,references/workflow-guide.md,references/skills-reference.md,references/config-reference.md}`
- Commands: `rg`, `sed -n`, `go run ./cmd/compozy --help`, `git status --short`
