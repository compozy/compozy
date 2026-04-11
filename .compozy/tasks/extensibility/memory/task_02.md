# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Build `internal/core/extension` as task 02's foundation package: manifest types, loader, validation, enablement model, and required tests only.
- Completed with manifest loading/validation, home-backed enablement persistence, and package tests plus full repository verification.

## Important Decisions

- Reuse existing repo dependencies for TOML parsing and semver comparison instead of adding new packages.
- Keep scope limited to loading/validation/state persistence; do not wire runtime discovery, subprocess startup, or event-bus integration in this task.
- Use `package extensions` inside `internal/core/extension` so the required `ExtensionInfo` type can exist without revive stutter violations.
- Decode manifests through raw TOML/JSON shapes so hook priority can default to `500` when omitted and JSON durations can be accepted as strings or integer nanoseconds.
- Store workspace enablement in `~/.compozy/state/workspace-extensions.json`, keyed by normalized workspace root, while user enablement lives beside the installed extension under `~/.compozy/extensions/<name>/.compozy-state.json`.

## Learnings

- Task 02 depends on hook names from `_protocol.md` section 6.5 and capability names from ADR-005, so both need single-source constants in the new package.
- Bundled extensions default enabled, while user and workspace extensions default disabled until explicitly enabled on the current machine.
- Validation needs to enforce capability-to-declaration relationships early: hook families require their matching mutate capability, provider declarations require `providers.register`, and skill resources require `skills.ship`.

## Files / Surfaces

- Implemented: `internal/core/extension/doc.go`
- Implemented: `internal/core/extension/manifest.go`
- Implemented: `internal/core/extension/manifest_load.go`
- Implemented: `internal/core/extension/manifest_validate.go`
- Implemented: `internal/core/extension/enablement.go`
- Implemented: `internal/core/extension/manifest_test.go`
- Implemented: `internal/core/extension/enablement_test.go`
- Updated: this task memory file

## Errors / Corrections

- Initial `make verify` failed on revive stutter rules and `Save` cyclomatic complexity; fixed by renaming the package identifier to `extensions`, shortening exported enablement helper types to `Source`/`Ref`, and splitting user/workspace persistence into dedicated helpers.
## Ready for Next Run

- Verification evidence:
  - `go test ./internal/core/extension -cover` => pass, `82.0%` coverage
  - `make verify` => pass, including lint, `1190` tests, and build
- Local code commit created: `aafb6a6` (`feat: scaffold extension manifest and enablement foundation`)
- Tracking-only files (`task_02.md`, `_tasks.md`, task memory, ledger) were intentionally left unstaged after update.
