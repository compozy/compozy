# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Move newly prepared runtime artifacts for existing PRD-task and review flows from `.tmp/codex-prompts` to workspace-scoped `.compozy/runs/<run-id>/jobs/`, while keeping preparation semantics, ordering, and filtering unchanged.

## Important Decisions
- Shared run-layout helpers live in `internal/core/model` via `RunsBaseDirForWorkspace`, `RunArtifacts`, and per-job artifact path helpers.
- Planner preparation now allocates one run directory per preparation and passes a shared `RunArtifacts` value into job building instead of hardcoding an artifact root in `input.go`.
- `SolvePreparation` now carries the allocated run-artifact metadata so later tasks can build on the same run-level seam.
- `runID` generation stays planner-owned and time-based for production runs, while tests assert the layout deterministically either with fixed run IDs or by checking the shared run directory structure.

## Learnings
- Existing runtime/executor layers only depend on per-job prompt/log paths, so the artifact-root migration stays isolated to shared model helpers and planner preparation.
- `agent.EnsureAvailable` already bypasses command checks for `DryRun`, which makes full `Prepare(...)` tests viable for both PRD-task and review modes without extra stubbing.

## Files / Surfaces
- `internal/cli/workspace_config_test.go`
- `internal/core/model/model.go`
- `internal/core/model/model_test.go`
- `internal/core/plan/input.go`
- `internal/core/plan/prepare.go`
- `internal/core/plan/prepare_test.go`
- `.codex/CONTINUITY-unified-run-artifacts.md`

## Errors / Corrections
- Removed an unused `filepath` import left behind after moving artifact allocation out of `input.go`.
- Hardened `model.NewRunArtifacts` to sanitize path separators in `runID` so shared helpers cannot accidentally create nested paths.
- `make verify` exposed an unrelated but real `internal/cli` race: parallel tests changed process cwd concurrently. Fixed it with a shared cwd test helper guarded by a mutex instead of weakening assertions or skipping the package.

## Ready for Next Run
- Remaining work is rerunning full-repo verification after the CLI test isolation fix, then final self-review and commit preparation if the gate stays clean.
