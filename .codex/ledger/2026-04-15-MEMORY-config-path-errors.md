## Goal (incl. success criteria):

- Verify the inline review finding about `resolveConfigPaths` swallowing global config path resolution errors, and fix it only if the current code still does that.
- Success criteria:
- Confirm current behavior from source and tests.
- Propagate `osUserHomeDir` and `resolveConfigBaseDir` failures through `resolveConfigPaths` and `loadEffectiveConfig`.
- Keep the change scoped to `internal/core/workspace`.
- Full verification passes via `make verify`.

## Constraints/Assumptions:

- Follow Go bug-fix workflow with `golang-pro`, `systematic-debugging`, and `no-workarounds`.
- Use `testing-anti-patterns` for test updates.
- Do not touch unrelated worktree changes or use destructive git commands.
- Only fix the finding if the current code still exhibits it.

## Key decisions:

- The finding is real in current `internal/core/workspace/config.go`: `resolveConfigPaths` returns `configPaths` only and silently drops `osUserHomeDir` / `resolveConfigBaseDir` failures by returning partial paths.
- Existing test `TestLoadConfigDoesNotRequireGlobalConfigWhenHomeLookupFails` codifies the buggy behavior and must be updated rather than preserved.

## State:

- Completed.

## Done:

- Read repository and workspace instructions.
- Loaded `golang-pro`, `systematic-debugging`, `no-workarounds`, and `testing-anti-patterns`.
- Read current `internal/core/workspace/config.go`, related helpers, and current tests.
- Verified the reported bug still exists in the current code.
- Updated `resolveConfigPaths` to return `(configPaths, error)` and propagate `osUserHomeDir` / `resolveConfigBaseDir` failures with context.
- Updated `loadEffectiveConfig` to wrap path resolution failures as `resolve config paths: %w`.
- Replaced the old optional-home behavior test with explicit error-propagation coverage for both failure branches.
- Fixed two environment-coupled workspace config tests so they isolate `HOME` instead of depending on the developer machine's real global config.
- Ran `go test ./internal/core/workspace -count=1` successfully.
- Ran `make verify` successfully.

## Now:

- None.

## Next:

- None.

## Open questions (UNCONFIRMED if needed):

- None.

## Working set (files/ids/commands):

- `.codex/ledger/2026-04-15-MEMORY-config-path-errors.md`
- `internal/core/workspace/config.go`
- `internal/core/workspace/config_test.go`
- `internal/core/workspace/config_merge.go`
- `go test ./internal/core/workspace -count=1`
- `make verify`
