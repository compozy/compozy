# TechSpec: `compozy exec`

## Executive Summary

This specification adds a new `compozy exec` command that runs an ad hoc prompt against any supported ACP runtime using the same execution stack that already powers `compozy start` and `compozy fix-reviews`. No `_prd.md` exists for this feature, so the design is derived from the user request and the current runtime architecture. The command supports explicit runtime selection such as `--ide codex --model gpt-5.4`, prompt input from positional args, `--prompt-file`, or `stdin`, and output selection through `--format text|json`.

The primary technical decision is to make ad hoc execution a first-class runtime mode instead of a command-specific shortcut. That choice requires a focused refactor in preparation and artifact layout, but it avoids duplicating ACP bootstrap, retries, logging, usage aggregation, and failure handling. The same refactor also retires the legacy `.tmp/codex-prompts` path and replaces it with a workspace-scoped `.compozy/runs/<run-id>/` layout shared by existing directory-backed runs and the new ad hoc command. The main trade-off is modest core refactoring now in exchange for a simpler and more coherent runtime model later.

## System Architecture

### Component Overview

`compozy exec` extends the current CLI-to-runtime path instead of creating a new one.

- `internal/cli`
Adds `newExecCommand()` and an `execCommandState` that resolves prompt source, applies workspace config precedence, validates `--format`, and builds a shared `core.Config`.

- `internal/core/api.go`
Extends the public config surface with a third execution mode and ad hoc prompt fields. `core.Run()` remains the single end-to-end entry point.

- `internal/core/model`
Adds execution-mode and output-format enums, ad hoc prompt input fields, and shared helpers for `.compozy/runs/`.

- `internal/core/plan`
Refactors preparation so it can build jobs from either directory-backed workflow inputs or a direct prompt payload. This package also allocates a run directory and job artifact paths for every invocation.

- `internal/core/run`
Reuses the current executor. Text mode keeps the existing human-oriented presentation path. JSON mode suppresses the TUI path and emits a machine-readable run result backed by the same ACP session updates and usage accounting.

- `internal/core/workspace`
Extends `.compozy/config.toml` parsing with an `[exec]` section and preserves precedence `flags > [exec] > [defaults] > internal defaults`.

- ACP runtimes
No new integration shape is introduced. `exec` uses the same runtime registry, bootstrap args, access mode, model selection, retries, and shutdown semantics already implemented for other modes.

Data flow:

1. CLI resolves prompt source, format, and runtime flags.
2. Workspace config is loaded and merged using existing precedence rules.
3. `core.Run()` validates the resulting runtime config.
4. `plan.Prepare()` creates a run directory under `.compozy/runs/<run-id>/` and builds one ad hoc job.
5. `run.Execute()` runs the job through the existing ACP executor.
6. Text mode renders the normal human view and writes artifacts.
7. JSON mode suppresses human UI output, writes artifacts, and emits a stable JSON result.

## Implementation Design

### Core Interfaces

```go
type ExecutionMode string

const (
    ExecutionModePRReview ExecutionMode = "pr-review"
    ExecutionModePRDTasks ExecutionMode = "prd-tasks"
    ExecutionModeExec     ExecutionMode = "exec"
)

type OutputFormat string

const (
    OutputFormatText OutputFormat = "text"
    OutputFormatJSON OutputFormat = "json"
)
```

```go
type RuntimeConfig struct {
    WorkspaceRoot   string
    Mode            ExecutionMode
    IDE             string
    Model           string
    OutputFormat    OutputFormat
    PromptText      string
    PromptFile      string
    ReadPromptStdin bool
}
```

```go
type RunArtifacts struct {
    RunID       string
    RunDir      string
    RunMetaPath string
    JobsDir     string
    ResultPath  string
}
```

```go
type ExecResult struct {
    RunID        string      `json:"run_id"`
    Mode         string      `json:"mode"`
    Status       string      `json:"status"`
    IDE          string      `json:"ide"`
    Model        string      `json:"model"`
    ArtifactsDir string      `json:"artifacts_dir"`
    Jobs         []JobResult `json:"jobs"`
}
```

### Data Models

Core additions:

- `model.ExecutionModeExec`
Represents ad hoc prompt execution.

- `model.OutputFormat`
Defines `text` and `json`.

- `model.RuntimeConfig` fields for:
`OutputFormat`, `PromptText`, `PromptFile`, `ReadPromptStdin`

- `model.Job` continues to carry prompt bytes, system prompt, and per-job artifact paths.
Ad hoc mode still produces a normal job so the executor stays generic.

- `RunArtifacts` metadata
Tracks the allocated `.compozy/runs/<run-id>/` directory and file paths used by preparation and execution.

- `ExecResult` / `JobResult`
Provides the JSON contract for `--format json`, including run metadata, per-job status, usage, exit information, and artifact paths.

Run artifact layout:

- `.compozy/runs/<run-id>/run.json`
- `.compozy/runs/<run-id>/jobs/<safe-name>.prompt.md`
- `.compozy/runs/<run-id>/jobs/<safe-name>.out.log`
- `.compozy/runs/<run-id>/jobs/<safe-name>.err.log`
- `.compozy/runs/<run-id>/result.json` for JSON-mode runs

Prompt source resolution rules:

- Positional prompt is accepted for short inline usage.
- `--prompt-file` is accepted for long or reusable prompts.
- `stdin` is used only when neither positional prompt nor `--prompt-file` is provided.
- Ambiguous combinations are rejected with explicit validation errors rather than silently guessed.

### API Endpoints

Not applicable. This feature adds a CLI surface and shared runtime internals, not an HTTP API.

## Integration Points

- ACP runtime registry
`exec` uses the existing runtime registry in `internal/core/agent/registry.go`, including model defaults, access-mode bootstrap args, fallback launchers, and availability checks.

- Workspace config
A new `[exec]` section extends `.compozy/config.toml` without changing the existing merge model.

- Standard input
`exec` reads prompt content from `stdin` only as an explicit fallback source, not as an always-on side channel.

- Filesystem artifacts
All modes write run artifacts under `.compozy/runs/`, replacing `.tmp/codex-prompts`.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `internal/cli/root.go` | modified | Adds `exec` command, prompt-source validation, and format handling. Medium risk because CLI contract changes. | Add command wiring, help text, and tests |
| `internal/core/api.go` | modified | Adds exec-mode config and shared run metadata. Medium risk due to public internal facade changes. | Extend config validation and runtime conversion |
| `internal/core/model/model.go` | modified | Adds mode/format enums and run path helpers. Low risk. | Introduce shared constants and helpers |
| `internal/core/workspace/config.go` | modified | Adds `[exec]` config support. Medium risk because precedence must remain stable. | Extend config structs and validation |
| `internal/core/plan/*` | modified | Refactors preparation to support prompt-backed runs and unified artifact layout. High risk because existing run modes depend on it. | Extract shared run-artifact allocation and add exec preparation path |
| `internal/core/run/*` | modified | JSON output suppresses UI and emits result payload. Medium risk because executor output flow changes. | Add output-format branching without duplicating execution logic |
| Existing tests | modified | Path and output expectations change. Medium risk. | Update path assertions and add coverage for exec mode |

## Testing Approach

### Unit Tests

- Prompt source resolution:
positional prompt, `--prompt-file`, `stdin`, and ambiguous combinations.
- Output format validation:
accept `text|json`, reject unsupported values.
- Workspace config precedence:
`flags > [exec] > [defaults] > internal defaults`.
- Run artifact path helpers:
`.compozy/runs/<run-id>/...` layout generation and deterministic job filenames in tests.
- JSON result assembly:
stable payload shape, status mapping, usage aggregation, and artifact paths.

### Integration Tests

- CLI command execution:
validate args, `stdin` behavior, file input, and help text.
- Shared executor behavior:
`--format text` keeps current runtime flow; `--format json` suppresses UI-oriented output but still writes prompt/log artifacts and emits `result.json`.
- Regression coverage for existing modes:
`start` and `fix-reviews` continue to allocate artifacts under the new shared root and preserve current execution semantics.
- Failure behavior:
ACP setup failures, runtime failures, and retryable failures still produce logs and machine-readable results consistently.

## Development Sequencing

### Build Order

1. Extend shared runtime models and workspace config to add `ExecutionModeExec`, `OutputFormat`, prompt-source fields, and `[exec]` defaults. No dependencies.
2. Replace `.tmp/codex-prompts` with shared `.compozy/runs/` helpers and run-artifact allocation. Depends on step 1.
3. Refactor planner preparation to support prompt-backed runs and emit one ad hoc job for `exec`. Depends on steps 1 and 2.
4. Add the CLI `exec` command, prompt-source validation, and config merge behavior. Depends on steps 1 and 3.
5. Extend executor output handling to support `--format text|json` and machine-readable result emission. Depends on steps 2 and 3.
6. Add regression coverage for unit, CLI, and integration scenarios across all three modes. Depends on steps 2, 3, 4, and 5.
7. Update docs/help text and remove remaining references to `.tmp/codex-prompts`. Depends on steps 4 and 5.

### Technical Dependencies

- Existing ACP executor and runtime registry must remain the only runtime transport path.
- Workspace config validation must stay strict and backward-compatible for existing sections.
- Run directory allocation needs deterministic seams for tests even if production run IDs include timestamps or random suffixes.

## Monitoring and Observability

- Structured log fields:
`run_id`, `mode`, `output_format`, `prompt_source`, `ide`, `model`, `artifacts_dir`, `job_safe_name`
- Run metadata:
`run.json` should record resolved config minus sensitive prompt duplication beyond the prompt artifact itself.
- JSON mode:
`result.json` and stdout JSON should be sufficient for automation to detect success, failure, retries, usage, and artifact locations.
- Failure investigation:
operators should be able to jump from a run ID directly to prompt, stdout log, and stderr log inside `.compozy/runs/<run-id>/`.

## Technical Considerations

### Key Decisions

- Decision: Model ad hoc execution as a first-class runtime mode.
Rationale: keeps ACP bootstrap, retries, logging, and shutdown behavior shared.
Trade-offs: requires planner/model refactor instead of a command-local shortcut.
Alternatives rejected: standalone job builder in CLI; direct ACP command path.

- Decision: Replace `.tmp/codex-prompts` with `.compozy/runs/`.
Rationale: removes a codex-specific legacy path and gives all execution modes one canonical artifact root.
Trade-offs: test fixtures and path assertions must change.
Alternatives rejected: keep legacy path; use `.compozy/runtime/runs/`.

- Decision: Support positional prompt, `--prompt-file`, or `stdin`, plus `--format text|json`.
Rationale: covers human usage, automation, and long-form prompt workflows with an explicit output contract.
Trade-offs: input validation and result-schema maintenance become stricter responsibilities.
Alternatives rejected: positional-only input; text-only output.

### Known Risks

- Preparation refactor risk:
breaking `start` or `fix-reviews` while adding `exec`.
Mitigation: keep job construction generic and add regression tests before removing old path assumptions.

- JSON contract creep:
automation may start depending on unstable fields.
Mitigation: keep the schema small, explicit, and artifact-path oriented.

- Artifact growth:
`.compozy/runs/` can accumulate files over time.
Mitigation: keep retention policy out of v1, but document the directory and make future cleanup policy possible.

- Prompt-source ambiguity:
mixing positional prompt, file input, and piped stdin can create surprising behavior.
Mitigation: reject ambiguous combinations with clear CLI errors.

## Architecture Decision Records

- [ADR-001: Model Ad Hoc Execution as a First-Class Runtime Mode](adrs/adr-001.md) — Adds `exec` by extending the shared planner and executor instead of building a parallel ACP path.
- [ADR-002: Replace `.tmp/codex-prompts` with Workspace-Scoped Run Artifacts](adrs/adr-002.md) — Unifies all execution artifacts under `.compozy/runs/<run-id>/`.
- [ADR-003: Support Multi-Source Prompt Input and Structured Output for `compozy exec`](adrs/adr-003.md) — Defines prompt-source resolution, `--format text|json`, and config precedence for the new command.
