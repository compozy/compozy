# CLI Subcommand Migration: `fix-reviews` and `start`

## Summary

- Replace the root-level `--mode pr-review|prd-tasks` switch with two explicit Cobra subcommands:
  - `looper fix-reviews`
  - `looper start`
- Make this a clean breaking CLI change:
  - remove `--mode` entirely from help and parsing
  - `looper` becomes a help-only parent command
  - interactive mode moves to `looper fix-reviews --form` and `looper start --form`
- Keep the internal and public Go API mode model intact for library callers; only the CLI surface changes.

## Implementation Changes

- Refactor `internal/cli/root.go` to build a fresh Cobra command tree per `NewRootCommand()` call instead of reusing one global root command with shared package state.
- Define a root command with no execution path of its own; running bare `looper` should print help and exit successfully.
- Add two leaf commands with explicit workflow ownership:
  - `fix-reviews` sets `core.ModePRReview`
  - `start` sets `core.ModePRDTasks`
- Split flag registration into shared flags plus workflow-specific flags.
  - Shared on both subcommands: `--dry-run`, `--auto-commit`, `--concurrent`, `--ide`, `--model`, `--add-dir`, `--tail-lines`, `--reasoning-effort`, `--form`, `--timeout`, `--max-retries`, `--retry-backoff-multiplier`
  - `fix-reviews` only: `--pr`, `--issues-dir`, `--batch-size`, `--grouped`
  - `start` only: `--name`, `--tasks-dir`, `--include-completed`
- Remove CLI-only prechecks that assume `--pr` is always required, and make subcommands build `core.Config` directly with the correct mode.
- Preserve `core.Config.Mode`, `looper.ModePRReview`, and `looper.ModePRDTasks` for embedders and Go package usage; the subcommands should set `Mode` explicitly when building config.
- Update the form flow in `internal/cli/form.go`:
  - remove the mode selector entirely
  - make `fix-reviews --form` show only review fields
  - make `start --form` show only PRD-task fields
  - keep the existing Huh/Bubble Tea theme and interaction model; no layout redesign is needed
- Update input resolution in `internal/looper/plan/input.go` so PRD runs can infer the task name from `tasks/prd-<name>` when `--tasks-dir` is provided without `--name`, mirroring the existing review-side directory inference.
- Replace mode/flag-specific error text that would leak old CLI wording with workflow-appropriate messaging for review vs PRD task execution.

## Public CLI / Interface Changes

- New CLI entrypoints:
  - `looper fix-reviews`
  - `looper start`
- Removed CLI flag:
  - `--mode`
- New `start` flag names:
  - `--name`
  - `--tasks-dir`
- Root behavior:
  - `looper` shows help
  - `looper --form` is no longer a valid workflow entrypoint
  - users must run `looper fix-reviews --form` or `looper start --form`
- No planned changes to the embeddable Go API beyond the root Cobra command now exposing subcommands.

## Test Plan

- Add/adjust CLI tests to verify:
  - root command exposes `fix-reviews` and `start`
  - root help no longer mentions `--mode`
  - bare root shows help successfully
- Add subcommand help tests to verify:
  - `fix-reviews` exposes review flags and omits PRD-only flags
  - `start` exposes `--name`, `--tasks-dir`, `--include-completed` and omits review-only flags
- Add form-related tests for workflow-specific field behavior:
  - `start` uses task-oriented labels/flag application
  - `fix-reviews` retains PR-review labels/flag application
  - no form path depends on a mode selector anymore
- Add input resolution tests for:
  - inferring review PR from `ai-docs/reviews-pr-<PR>/issues`
  - inferring PRD name from `tasks/prd-<name>`
  - clear errors when neither identifier nor directory is usable
- Update docs/examples tests and run full verification with `make verify`.

## Assumptions And Defaults

- `--mode` is removed completely, with no deprecation shim or hidden compatibility path.
- The root command is a parent-only container and does not default to `start`.
- `start` uses `--name` and `--tasks-dir` as the canonical PRD-task flags.
- Bubble Tea changes are limited to the Huh form field set and copy; no run-screen layout changes are required.
- Checked-in CLI-facing docs and examples that still show `--mode` should be updated, including README content and any tracked reference docs that present current usage.
