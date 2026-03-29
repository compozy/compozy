# Technical Specification: Review Workflow Refactor

## Executive Summary

Refactor the PR review workflow to decouple it from CodeRabbit and make it provider-agnostic. The current `fix-reviews` command, `fix-coderabbit-review` skill, and associated Python/Bash scripts are tightly coupled to CodeRabbit's review format and `ai-docs/` directory structure. This refactor introduces a Go-native provider abstraction for fetching and resolving reviews, a new `fetch-reviews` CLI command, a standardized review file format (`issue_NNN.md`), and a generic `fix-reviews` skill. Review files move into the PRD directory structure (`tasks/prd-<name>/reviews-NNN/`) with support for multiple review rounds. Post-execution thread resolution becomes automatic in the looper pipeline instead of being delegated to the LLM.

## System Architecture

### Component Overview

```
CLI Layer
  looper fetch-reviews --provider coderabbit --pr 259 --name my-feature [--round N]
  looper fix-reviews   --name my-feature [--round N] [--reviews-dir path] [--batch-size N] ...

Provider Layer (new)
  internal/looper/provider/
    provider.go       → Provider interface (FetchReviews + ResolveIssues)
    registry.go       → Provider registry (string → Provider)
    coderabbit/
      coderabbit.go   → CodeRabbit implementation (migrates Python export + Bash resolve)

Plan Layer (modified)
  internal/looper/plan/
    input.go          → New readReviewEntries() + resolveReviewDir() + round discovery
    prepare.go        → Updated to pass provider context through the pipeline

Prompt Layer (modified)
  internal/looper/prompt/
    review.go         → References generic `fix-reviews` skill, no CodeRabbit mentions
    common.go         → New ExtractIssueNumber() for issue_NNN.md pattern

Execution Layer (modified)
  internal/looper/run/
    execution.go      → Post-job hook: call provider.ResolveIssues() for resolved issues

Skill Layer (replaced)
  skills/fix-reviews/SKILL.md → Generic 4-step workflow (read, triage, fix, verify)
  skills/fix-coderabbit-review/ → DELETED
```

### Data Flow

```
fetch-reviews:
  CLI → Provider.FetchReviews(pr) → []ReviewItem → writer → tasks/prd-<name>/reviews-NNN/

fix-reviews:
  CLI → plan.Prepare() → read issue files → batch → prompt → agent execution
      → post-job: parse resolved issues → Provider.ResolveIssues() → GitHub API
```

## Implementation Design

### Core Interfaces

```go
// internal/looper/provider/provider.go

// ReviewItem is the normalized output of a provider fetch operation.
type ReviewItem struct {
    Title       string // Short title extracted from the review comment
    File        string // Source file referenced by the review
    Line        int    // Line number in the source file
    Severity    string // error, warning, suggestion, nitpick (optional)
    Author      string // Comment author (e.g., "coderabbitai[bot]")
    Body        string // Full review comment body (markdown)
    ProviderRef string // Opaque provider reference (e.g., "thread:PRT_abc,comment:RC_123")
}

// ResolvedIssue identifies an issue file that the agent marked as resolved.
type ResolvedIssue struct {
    FilePath    string // Absolute path to the issue_NNN.md file
    ProviderRef string // Extracted from <provider_ref> in the issue file
}

// Provider abstracts review fetching and thread resolution for a specific source.
type Provider interface {
    // Name returns the provider identifier (e.g., "coderabbit").
    Name() string
    // FetchReviews retrieves review comments from the given PR.
    FetchReviews(ctx context.Context, pr string) ([]ReviewItem, error)
    // ResolveIssues marks resolved issues in the provider's external system.
    ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error
}
```

```go
// internal/looper/provider/registry.go

// Registry maps provider names to implementations.
type Registry struct {
    providers map[string]Provider
}

func NewRegistry() *Registry
func (r *Registry) Register(p Provider)
func (r *Registry) Get(name string) (Provider, error)
func DefaultRegistry() *Registry // returns registry with coderabbit pre-registered
```

### Data Models

**RuntimeConfig changes** (`internal/looper/model/model.go`):

```go
type RuntimeConfig struct {
    // existing fields...
    Name     string // PRD name (shared between start and fix-reviews)
    Round    int    // Review round number (0 = auto-detect latest)
    Provider string // Provider name for fetch-reviews (e.g., "coderabbit")

    // removed/deprecated:
    // PR string       → kept but semantics change: set by fetch-reviews, read from _meta.md by fix-reviews
    // IssuesDir string → replaced by ReviewsDir for review mode
}
```

**New model types**:

```go
// RoundMeta represents the _meta.md file in a review round directory.
type RoundMeta struct {
    Provider  string    // Provider name that created this round
    PR        string    // Pull request number
    Round     int       // Round number
    CreatedAt time.Time // When the round was created
    Total     int       // Total issue count
    Resolved  int       // Resolved issue count
    Unresolved int      // Unresolved issue count
}
```

### File Formats

**`_meta.md`** (in `tasks/prd-<name>/reviews-NNN/`):

```markdown
---
provider: coderabbit
pr: 259
round: 1
created_at: 2026-03-28T10:00:00Z
---

## Summary
- Total: 15
- Resolved: 3
- Unresolved: 12
```

**`issue_NNN.md`** (standardized review issue file):

```markdown
# Issue 001: <title>

## Status: pending

<review_context>
  <file>internal/looper/plan/prepare.go</file>
  <line>42</line>
  <severity>warning</severity>
  <author>coderabbitai[bot]</author>
  <provider_ref>thread:PRT_kwDOAbc123,comment:RC_456789</provider_ref>
</review_context>

## Review Comment

<original review comment body in markdown>

## Triage

- Decision: `UNREVIEWED`
- Notes:
```

**Status lifecycle**: `pending` → `valid` | `invalid` → `resolved`

**`<review_context>` fields**:
| Field | Required | Description |
|-------|----------|-------------|
| `file` | yes | Source file path referenced by the review |
| `line` | yes | Line number in the source file |
| `severity` | no | `error`, `warning`, `suggestion`, `nitpick` |
| `author` | yes | Review comment author |
| `provider_ref` | no | Opaque provider reference for thread resolution |

### Directory Structure

```
tasks/prd-<name>/
  _prd.md
  _techspec.md
  _tasks.md
  task_01.md
  task_02.md
  reviews-001/                    # Round 1
    _meta.md                      # Provider metadata + summary counts
    issue_001.md                  # Standardized review issue format
    issue_002.md
    issue_003.md
    grouped/                      # Generated by fix-reviews when --grouped
      group_auth-middleware.md
      group_api-handler.md
  reviews-002/                    # Round 2 (subsequent review)
    _meta.md
    issue_001.md
    issue_002.md
```

**Naming conventions**:
- Round directory: `reviews-NNN` (zero-padded 3 digits)
- Issue files: `issue_NNN.md` (zero-padded 3 digits)
- Meta document: `_meta.md` (underscore prefix, consistent with PRD conventions)
- Grouped directory: `grouped/` (generated on demand)

### CLI Commands

**`looper fetch-reviews`** (new command):

```
looper fetch-reviews --provider coderabbit --pr 259 --name my-feature [--round N]
```

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--provider` | string | yes | - | Provider name (`coderabbit`) |
| `--pr` | string | yes | - | Pull request number |
| `--name` | string | yes | - | PRD name (resolves to `tasks/prd-<name>/`) |
| `--round` | int | no | auto-increment | Round number; auto-detects next round if omitted |

Behavior:
1. Resolve PRD directory `tasks/prd-<name>/`
2. Determine round number (scan existing `reviews-NNN/` dirs, pick next)
3. Call `provider.FetchReviews(ctx, pr)` to get `[]ReviewItem`
4. Write `_meta.md` with provider, PR, round, and counts
5. Write `issue_NNN.md` for each `ReviewItem` in standardized format
6. Print summary: directory path, issue count, round number

**`looper fix-reviews`** (refactored command):

```
looper fix-reviews --name my-feature [--round N] [--reviews-dir path] [--batch-size N] [--grouped] ...
```

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--name` | string | yes* | - | PRD name |
| `--round` | int | no | latest | Round number; defaults to latest existing round |
| `--reviews-dir` | string | no | - | Override: direct path to reviews directory |
| `--batch-size` | int | no | 1 | Number of file groups per batch |
| `--grouped` | bool | no | false | Generate grouped issue summaries |
| `--include-resolved` | bool | no | false | Include already-resolved issues |

*`--name` is required unless `--reviews-dir` is provided.

Shared flags (unchanged): `--ide`, `--model`, `--dry-run`, `--auto-commit`, `--concurrent`, `--add-dir`, `--tail-lines`, `--reasoning-effort`, `--timeout`, `--max-retries`, `--retry-backoff-multiplier`, `--form`.

**Removed flags**: `--pr` (read from `_meta.md`), `--issues-dir` (replaced by `--reviews-dir`).

## Integration Points

### GitHub API (via `gh` CLI)

The CodeRabbit provider needs:
- **Fetch**: `gh api repos/{owner}/{repo}/pulls/{pr}/comments` (REST) + GraphQL for thread data
- **Resolve**: GraphQL mutation `resolveReviewThread` per thread ID

Both operations require `gh` authenticated with `repo` scope. The provider implementation calls `gh` as a subprocess (same as current Python/Bash scripts) to avoid adding GitHub API dependencies.

### Provider Resolution in Execution Pipeline

After each job completes successfully, the execution pipeline:
1. Re-reads the issue files from the job's batch
2. Identifies issues whose status changed to `resolved`
3. Extracts `provider_ref` from each resolved issue's `<review_context>`
4. Calls `provider.ResolveIssues()` with the resolved issues
5. Updates `_meta.md` counts

This requires the execution pipeline to receive provider context. The `RuntimeConfig` carries the provider name; the `_meta.md` carries the PR number.

## Impact Analysis

| Affected Component | Type of Impact | Description & Risk Level | Required Action |
|---|---|---|---|
| `internal/cli/root.go` | Command restructure | `fix-reviews` flags change, new `fetch-reviews` command added. Medium risk. | Update flag registration, add new command builder |
| `internal/looper/api.go` | API change | `Config` struct gains `Name`, `Round`, `Provider` fields; `PR` semantics change. Medium risk. | Update public API, ensure backward compatibility not needed (CLI-only consumer) |
| `internal/looper/model/model.go` | Model change | `RuntimeConfig` gains new fields, `RoundMeta` type added. Low risk. | Add fields, add `RoundMeta` |
| `internal/looper/plan/input.go` | Logic rewrite | Review input resolution changes from `ai-docs/` to `tasks/prd-<name>/reviews-NNN/`. High risk. | Rewrite `resolveInputs()` for review mode, add round discovery |
| `internal/looper/plan/prepare.go` | Pipeline change | Must pass provider context through to execution. Medium risk. | Thread provider info into `SolvePreparation` |
| `internal/looper/prompt/review.go` | Prompt rewrite | Remove all CodeRabbit references, reference `fix-reviews` skill. Medium risk. | Rewrite prompt builder |
| `internal/looper/prompt/common.go` | New parser | Add `ExtractIssueNumber()` for `issue_NNN.md` pattern, `ParseReviewContext()`. Low risk. | Add new functions |
| `internal/looper/run/execution.go` | Post-job hook | Add provider resolve call after successful job completion. Medium risk. | Add post-success hook in job lifecycle |
| `skills/fix-coderabbit-review/` | Deletion | Entire skill directory removed. Low risk. | Delete directory |
| `skills/fix-reviews/` | New skill | Generic 4-step review remediation skill. Low risk. | Create new skill |

## Testing Approach

### Unit Tests

- **Provider interface**: test CodeRabbit provider with mocked `gh` output (JSON fixtures)
- **Registry**: test registration, lookup, missing provider error
- **File writer**: test `issue_NNN.md` generation from `ReviewItem` (format correctness, zero-padding, XML context)
- **Meta parser**: test `_meta.md` reading/writing, round discovery logic
- **Input resolution**: test `resolveReviewDir()` with various round configurations
- **Issue parser**: test `ExtractIssueNumber()`, `ParseReviewContext()`, status detection
- **Prompt builder**: test new review prompt references `fix-reviews` skill, no CodeRabbit mentions
- **Post-job resolver**: test that resolved issues are correctly identified and passed to provider

### Integration Tests

- **End-to-end fetch**: mock provider returns items, verify directory structure and file contents
- **End-to-end fix (dry-run)**: verify prompt generation with new format
- **Round auto-increment**: create `reviews-001/`, verify next fetch creates `reviews-002/`

## Development Sequencing

### Build Order

1. **Provider interface + registry + CodeRabbit implementation** — Foundation; no existing code depends on it yet. Migrate logic from Python export script (`export_coderabbit_review.py`) and Bash resolve script (`resolve_pr_issues.sh`) into Go.

2. **File format: writer + parser** — Write `issue_NNN.md` and `_meta.md` from `ReviewItem`; parse them back. This establishes the new file format that all subsequent components consume.

3. **`fetch-reviews` CLI command** — New command, no existing code changes. Wires provider + writer to produce review directories in `tasks/prd-<name>/reviews-NNN/`.

4. **Plan layer refactor** (`input.go`, `prepare.go`) — Update input resolution for review mode to read from `tasks/prd-<name>/reviews-NNN/` instead of `ai-docs/`. Add round discovery. Thread provider context into `SolvePreparation`.

5. **Prompt layer refactor** (`review.go`, `common.go`) — Update prompts to reference `fix-reviews` skill, use `<review_context>` XML for code file extraction, add `ExtractIssueNumber()`.

6. **Execution layer: post-job resolve hook** (`execution.go`) — After successful job completion, identify resolved issues and call `provider.ResolveIssues()`. Update `_meta.md` counts.

7. **`fix-reviews` CLI command refactor** (`root.go`) — Replace `--pr`/`--issues-dir` flags with `--name`/`--round`/`--reviews-dir`. Wire new input resolution.

8. **`fix-reviews` skill** — Create `skills/fix-reviews/SKILL.md` with generic 4-step workflow.

9. **Cleanup** — Delete `skills/fix-coderabbit-review/` directory (SKILL.md, scripts/). Remove `ai-docs/` references from codebase. Update README.

### Technical Dependencies

- `gh` CLI must be available and authenticated (existing requirement, no change)
- No new external Go dependencies needed; provider calls `gh` via `os/exec`

## Monitoring & Observability

- **Fetch**: log provider name, PR number, issue count, round number, output directory via `slog`
- **Fix**: existing job execution logging applies; add log line for post-job resolve (issues resolved count, provider errors)
- **Resolve errors**: non-fatal per-issue; log each failure with thread ID and error, continue with remaining issues

## Technical Considerations

### Key Decisions

1. **Provider calls `gh` as subprocess instead of using GitHub Go SDK** — Avoids adding `google/go-github` dependency; `gh` handles authentication, rate limiting, and pagination. The current scripts already depend on `gh`. Trade-off: slightly less control over error handling, but simpler dependency tree.

2. **Post-job resolve in execution pipeline instead of in-skill** — Makes resolution deterministic and removes LLM involvement in API calls. The agent focuses on code changes; the pipeline handles external state. Trade-off: tighter coupling between execution layer and provider, but more reliable.

3. **`issue_NNN.md` naming instead of `NNN-slug.md`** — Consistent with `task_NNN.md` pattern in PRD tasks. Simpler to parse, sort, and reference. Trade-off: less human-readable filenames, but the title is in the file header.

4. **`<review_context>` XML instead of markdown headers** — Consistent with `<task_context>` in task files. Machine-parseable without ambiguity. The `provider_ref` field is opaque to the format but structured for the provider.

5. **Round directories instead of flat files** — Supports iterative review cycles without file conflicts. Each round is a clean snapshot. The `_meta.md` per round captures the context at fetch time.

### Known Risks

1. **CodeRabbit API changes** — The provider relies on GitHub's review comment API filtering by `coderabbitai[bot]` author. If CodeRabbit changes its bot username, the provider needs updating. Mitigation: make the author filter configurable in the provider.

2. **Large PR review sets** — A PR with hundreds of review comments could generate many files. Mitigation: the existing batching mechanism handles this; `fetch-reviews` just writes files, `fix-reviews` batches them.

3. **Concurrent resolve race conditions** — Multiple concurrent jobs could try to resolve overlapping threads. Mitigation: each job only resolves its own batch's issues; batches are non-overlapping by construction.

4. **Migration from `ai-docs/`** — Existing users with `ai-docs/reviews-pr-*` directories will need to re-fetch using the new command. Mitigation: document in changelog; old directories are not deleted automatically.
