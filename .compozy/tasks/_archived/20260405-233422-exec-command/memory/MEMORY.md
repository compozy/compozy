# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- Task 01 completed the shared runtime/config contract for `exec`; later tasks can build command/artifact behavior on top of existing `core.Config`, `model.RuntimeConfig`, and workspace config support.

## Shared Decisions
- `exec` is represented as a first-class execution mode in both `internal/core/api.go` and `internal/core/model/model.go`.
- Shared runtime config now carries `OutputFormat`, `PromptText`, `PromptFile`, and `ReadPromptStdin` so downstream layers do not need CLI-only prompt-source inference.
- Runtime config also carries `ResolvedPromptText`, which lets CLI resolve stdin or prompt-file contents once while preserving the original raw prompt-source fields for validation and metadata.
- Workspace config precedence for future `exec` command state is `flags > [exec] > [defaults] > internal defaults`, reusing the existing CLI merge path rather than a parallel resolver.
- Shared run-artifact layout now lives in `internal/core/model` via `RunArtifacts` and `RunsBaseDirForWorkspace`; planner preparation allocates one `.compozy/runs/<run-id>/jobs/` directory per run instead of using `.tmp/codex-prompts`.
- `model.SolvePreparation` now carries `RunArtifacts`, giving later tasks a stable run-level seam without changing executor job contracts.

## Shared Learnings
- Runtime validation now treats prompt source and JSON output as `exec`-only concerns: non-`exec` modes must stay on text output and cannot set prompt-source fields.
- The repo’s committed CLI fixtures currently live partly under `.compozy/tasks/_archived/`; tests that assume active-task paths must resolve fixtures by committed location rather than hard-coded active paths.
- Existing runtime execution code already consumes only per-job prompt/log paths, so the run-root migration can stay isolated to shared model/planner layers.
- JSON-mode execution needs two separate switches: disable Bubble Tea UI and disable non-UI ACP stream mirroring to process stdout/stderr, while still writing the job log files.

## Open Risks
- Task 03 still needs to wire the actual Cobra command and prompt-source resolution onto the shared config surface added here.

## Handoffs
- Reuse `commandKindExec`, `commandState.outputFormat`, and the `[exec]` workspace section instead of introducing new command-local precedence logic in later tasks.
