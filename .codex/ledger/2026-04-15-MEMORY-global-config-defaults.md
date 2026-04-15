## Goal (incl. success criteria):

- Implement support for `~/.compozy/config.toml` as global defaults merged with workspace `.compozy/config.toml`.
- Success criteria:
- Global config uses the same schema as workspace config.
- Effective precedence is centralized and consistent across CLI and non-CLI consumers.
- Existing workspace-only behavior remains unchanged when no global config exists.
- Full verification passes via `make verify`.

## Constraints/Assumptions:

- Accepted implementation plan must be persisted under `.codex/plans/`.
- Use the existing `internal/core/workspace` package as the single source of truth for config loading.
- No destructive git commands; do not touch unrelated worktree changes.
- v1 path is exactly `~/.compozy/config.toml`; no XDG support in this change.
- Global config scope matches the full current schema: `defaults`, `exec`, `start`, `fix_reviews`, `fetch_reviews`, `tasks`, `sound`.

## Key decisions:

- Merge config centrally in `internal/core/workspace` instead of adding a second loader in CLI.
- Keep per-field precedence; list fields are replaced by the more specific scope rather than concatenated.
- Resolve config-owned relative `add_dirs` against the owning config root: workspace root for workspace config, home dir for global config.
- Preserve reusable-agent runtime precedence above config for exec runtime fields already controlled by `AGENT.md`.

## State:

- Completed.

## Done:

- Re-read repository instructions and mandatory skills for Go/tests/verification.
- Reconstructed the existing workspace config architecture and confirmed the root cause: only workspace config is loaded today.
- Mapped all current config sections, validation rules, CLI application points, and non-CLI consumers of `workspace.LoadConfig(...)`.
- Confirmed user decisions from planning:
- Full schema support in global config.
- Official path is `~/.compozy/config.toml` only.
- Produced a decision-complete implementation plan.
- Implemented central global+workspace config loading in `internal/core/workspace`.
- Added per-field config merge, source-aware validation labels, and config-owned `add_dirs` path normalization.
- Extended workspace context metadata to expose workspace/global config paths.
- Verified existing non-CLI consumers inherit merged config through `workspace.LoadConfig(...)` without bespoke changes.
- Added regression tests for:
- global fallback with no workspace config
- workspace-over-global field precedence
- cross-scope merged validation failures
- config-relative `add_dirs` resolution
- CLI application of global defaults and workspace overrides
- Updated CLI help text, README, reusable-agent docs, and start help golden fixture.
- Ran targeted verification:
- `go test ./internal/core/workspace`
- `go test ./internal/cli`
- `go test ./internal/core/...`
- Ran full verification: `make verify` (pass).
- Reviewed the landed change set against the documented precedence and identified two real defects:
- effective config composition let `global` command sections override `workspace` defaults
- optional global config loading failed when the home directory could not be resolved
- Refactored effective config composition to preserve precedence per command field:
- `workspace` command override > `workspace` defaults > `global` command override > `global` defaults
- command-only fields still merge `global` -> `workspace`
- Made global config discovery optional when `HOME` cannot be resolved; workspace config loading now continues.
- Added regression tests in `internal/core/workspace` and `internal/cli` for both precedence and optional-home behavior.
- Re-ran targeted verification:
- `go test ./internal/core/workspace -count=1`
- `go test ./internal/cli -count=1`
- Re-ran the full repository gate successfully:
- `make verify`
- Result: `0 issues`, `DONE 1905 tests in 14.215s`, successful build, final line `All verification checks passed`

## Now:

- None.

## Next:

- None.

## Open questions (UNCONFIRMED if needed):

- None.

## Working set (files/ids/commands):

- `.codex/plans/2026-04-15-global-config-defaults.md`
- `.codex/ledger/2026-04-15-MEMORY-global-config-defaults.md`
- `internal/core/workspace/config.go`
- `internal/core/workspace/config_merge.go`
- `internal/core/workspace/config_types.go`
- `internal/core/workspace/config_validate.go`
- `internal/cli/workspace_config.go`
- `internal/cli/workspace_config_test.go`
- `internal/cli/commands.go`
- `internal/cli/root.go`
- `README.md`
- `internal/core/migration/migrate.go`
- `internal/core/extension/host_helpers.go`
