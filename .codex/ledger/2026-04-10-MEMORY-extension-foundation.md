Goal (incl. success criteria):

- Implement task 02 by creating `internal/core/extension` with manifest types, TOML-first/JSON-fallback loader, validation (capabilities, hook events, priorities, min version), operator-local enablement storage, and tests.
- Success requires task-specific tests, package coverage >=80%, clean `make verify`, workflow memory/tracking updates, and one local commit.

Constraints/Assumptions:

- Follow repository `AGENTS.md`/`CLAUDE.md`, required skills, and task 02 scope only.
- Do not touch unrelated dirty worktree entries (`go.mod`, `go.sum`, existing `.compozy/tasks/extensibility/*` state) except required task memory/tracking files.
- Skip `brainstorming` design gating because the task already comes from an approved PRD/techspec workflow and explicitly requires `cy-execute-task`.

Key decisions:

- Use existing repo dependencies for TOML and semver handling rather than adding new ones.
- Mirror existing repository patterns for context-aware file IO, home-dir resolution, and warning logs.

State:

- Completed after fresh package coverage, full `make verify`, tracking updates, and local code commit `aafb6a6`.

Done:

- Read repository instructions, required skills, workflow memory, task 02 spec, `_tasks.md`, relevant techspec sections, ADR-005, ADR-007, and protocol hook taxonomy.
- Verified there are no blocking contradictions among the task spec and source documents.
- Inspected existing repo patterns for TOML decoding, semver parsing, home-directory state files, logging, subprocess APIs, and task-memory templates.
- Added `internal/core/extension` with `doc.go`, manifest types, capability and hook taxonomies, TOML-first/JSON-fallback loading, validation, and enablement persistence.
- Added tests covering TOML/JSON loading, precedence warnings, not-found/decode/validation failures, realistic manifest fixtures, default enablement, corrupt state handling, and enable/disable round-trips.
- Reworked the package identifier to `extensions` and split enablement persistence helpers to satisfy revive and gocyclo without suppressions.
- Ran `go test ./internal/core/extension -cover` successfully with `82.0%` coverage.
- Ran fresh `make verify` successfully: fmt, lint, `1190` tests, and build all passed.
- Updated `.compozy/tasks/extensibility/task_02.md` and `.compozy/tasks/extensibility/_tasks.md` to completed.
- Created local commit `aafb6a6` with the new extension package only; tracking files remain unstaged by design.

Now:

- Prepare the final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-extension-foundation.md`
- `.compozy/tasks/extensibility/task_02.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/_protocol.md`
- `.compozy/tasks/extensibility/adrs/adr-005.md`
- `.compozy/tasks/extensibility/adrs/adr-007.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_02.md`
- `internal/version/version.go`
- `internal/core/workspace/config.go`
- `internal/update/check.go`
- `internal/update/state.go`
- `internal/core/memory/store.go`
- `internal/core/provider/registry.go`
- `internal/core/subprocess/process.go`
- `internal/core/subprocess/handshake.go`
- `internal/core/extension/doc.go`
- `internal/core/extension/manifest.go`
- `internal/core/extension/manifest_load.go`
- `internal/core/extension/manifest_validate.go`
- `internal/core/extension/enablement.go`
- `internal/core/extension/manifest_test.go`
- `internal/core/extension/enablement_test.go`
- Commands: `rg`, `sed`, `git status --short`, `go test ./internal/core/extension -cover`, `make verify`
