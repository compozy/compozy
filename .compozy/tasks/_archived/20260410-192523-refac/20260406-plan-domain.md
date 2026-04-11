# Refactoring Analysis: Group 4 -- Plan, Prompt & Domain Packages

**Date**: 2026-04-06
**Scope**: `internal/core/plan`, `internal/core/prompt`, `internal/core/tasks`, `internal/core/memory`, `internal/core/reviews`, `internal/core/frontmatter`, `internal/core/workspace`, `internal/core/preputil`
**Total files analyzed**: 16 production Go files (~3,200 lines)

---

## Executive Summary

| Severity | Count |
|----------|-------|
| P0       | 3     |
| P1       | 8     |
| P2       | 9     |
| P3       | 4     |
| **Total**| **24**|

The most critical structural problem is the `prompt` package serving as a de-facto domain layer for both tasks and reviews parsing, creating an inverted dependency where domain stores (`tasks`, `reviews`) depend upward on a presentation/prompt package for core parsing logic. The second major issue is extensive cross-package duplication of error wrappers, dependency normalizers, and metadata summary parsers. The third is `prompt/common.go` acting as a "god file" mixing four unrelated concerns.

---

## Package Summaries

### `internal/core/plan` (2 files, ~778 lines)
Orchestrates preparation of workflow runs and exec runs. Resolves inputs, reads issue entries, groups and batches them, allocates run artifacts, and writes metadata. Heavy coupling to `prompt`, `tasks`, `reviews`, `memory`, `agent`, `journal`.

### `internal/core/prompt` (4 files, ~673 lines)
Mixed-responsibility package: task/review parsing, legacy format detection, prompt text generation, file naming utilities, template loading. Imported by 8+ packages. Acts as both a prompt builder and a domain parsing library.

### `internal/core/tasks` (6 files, ~503 lines)
Task store operations (meta read/write/refresh, mark complete), validation, type registry, title extraction, legacy type remapping, fix prompt generation. Depends on `prompt` for all parsing.

### `internal/core/reviews` (1 file, ~426 lines)
Review round store operations (discover/read/write rounds, read entries, finalize statuses, meta operations). Depends on `prompt` for all parsing.

### `internal/core/memory` (1 file, ~167 lines)
Workflow memory bootstrapping and inspection. Clean, cohesive, minimal dependencies. Well-structured.

### `internal/core/frontmatter` (1 file, ~124 lines)
Generic YAML frontmatter parser/formatter. Clean utility. No issues.

### `internal/core/workspace` (1 file, ~438 lines)
Workspace discovery, TOML config loading, and validation. Long but well-structured with a validator-per-field pattern.

### `internal/core/preputil` (1 file, ~40 lines)
Single-function utility for journal cleanup. Minimal.

---

## Detailed Findings

### F01 -- `prompt` package is a misplaced domain parsing layer (P0)

**Type**: Architectural boundary violation
**Action**: **(B) Package-level split**

The `prompt` package contains core domain parsing functions (`ParseTaskFile`, `ParseReviewContext`, `IsTaskCompleted`, `IsReviewResolved`, `ExtractTaskNumber`, `ExtractIssueNumber`) that are imported by `tasks/store.go`, `tasks/validate.go`, `reviews/store.go`, `plan/input.go`, `plan/prepare.go`, `run/execution.go`, and `migrate.go`.

This creates an inverted dependency: domain store packages (`tasks`, `reviews`) depend on a prompt/presentation package for fundamental parsing. The prompt package should only build prompt text, not own domain parsing.

**Evidence** (callers of `prompt.ParseTaskFile`):
- `internal/core/tasks/store.go:92` -- `prompt.ParseTaskFile(string(content))`
- `internal/core/tasks/store.go:213` -- `prompt.ParseTaskFile(string(body))`
- `internal/core/tasks/validate.go:114` -- `prompt.ParseTaskFile(content)`
- `internal/core/plan/prepare.go:231` -- `prompt.ParseTaskFile(batchIssues[0].Content)`
- `internal/core/plan/input.go:246` -- `prompt.ParseTaskFile(content)`
- `internal/core/run/execution.go:987` -- `prompt.ParseTaskFile(entry.Content)`

**Evidence** (callers of `prompt.ParseReviewContext`):
- `internal/core/reviews/store.go:130` -- `prompt.ParseReviewContext(content)`
- `internal/core/reviews/store.go:351` -- `prompt.ParseReviewContext(string(content))`
- `internal/core/run/execution.go:1874` -- `prompt.ParseReviewContext(currentContent)`

**Recommendation**: Extract task parsing (`ParseTaskFile`, `ParseLegacyTaskFile`, `IsTaskCompleted`, `ExtractTaskNumber`, `LooksLikeLegacyTaskFile`, `ExtractLegacyTaskBody`) into `tasks` package. Extract review parsing (`ParseReviewContext`, `ParseLegacyReviewContext`, `IsReviewResolved`, `ExtractIssueNumber`, `LooksLikeLegacyReviewFile`, `ExtractLegacyReviewBody`) into `reviews` package. Keep only prompt-building functions in `prompt`.

---

### F02 -- Duplicated `wrapTaskParseError` function (P0)

**Type**: DRY violation (copy-pasted function)
**Action**: **(C) Extraction**

The exact same function exists in two packages:

`internal/core/plan/input.go:325-330`:
```go
func wrapTaskParseError(path string, err error) error {
    if errors.Is(err, prompt.ErrLegacyTaskMetadata) || errors.Is(err, prompt.ErrV1TaskMetadata) {
        return fmt.Errorf("legacy task artifact detected at %s; run `compozy migrate`", path)
    }
    return fmt.Errorf("parse task artifact %s: %w", path, err)
}
```

`internal/core/tasks/store.go:225-230`:
```go
func wrapTaskParseError(path string, err error) error {
    if errors.Is(err, prompt.ErrLegacyTaskMetadata) || errors.Is(err, prompt.ErrV1TaskMetadata) {
        return fmt.Errorf("legacy task artifact detected at %s; run `compozy migrate`", path)
    }
    return fmt.Errorf("parse task artifact %s: %w", path, err)
}
```

**Recommendation**: Move to the `tasks` package as a single exported `WrapParseError` function when domain parsing is relocated there (see F01). The `plan` package would then call `tasks.WrapParseError(...)`.

---

### F03 -- Duplicated `wrapReviewParseError` function (P0)

**Type**: DRY violation (copy-pasted function)
**Action**: **(C) Extraction**

Same pattern as F02:

`internal/core/plan/input.go:332-337`:
```go
func wrapReviewParseError(path string, err error) error {
    if errors.Is(err, prompt.ErrLegacyReviewMetadata) {
        return fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", path)
    }
    return fmt.Errorf("parse review artifact %s: %w", path, err)
}
```

`internal/core/reviews/store.go:420-425`:
```go
func wrapReviewParseError(path string, err error) error {
    if errors.Is(err, prompt.ErrLegacyReviewMetadata) {
        return fmt.Errorf("legacy review artifact detected at %s; run `compozy migrate`", path)
    }
    return fmt.Errorf("parse review artifact %s: %w", path, err)
}
```

**Recommendation**: Consolidate into `reviews.WrapParseError(...)`.

---

### F04 -- Duplicated `normalizeDependencies` function (P1)

**Type**: DRY violation
**Action**: **(C) Extraction**

`internal/core/prompt/common.go:472-489` (`normalizeDependencies`) and `internal/core/tasks/validate.go:150-166` (`normalizeValidationDependencies`) are structurally identical:

```go
// prompt/common.go:472
func normalizeDependencies(values []string) []string {
    normalized := make([]string, 0, len(values))
    for _, value := range values {
        trimmed := strings.TrimSpace(value)
        if trimmed == "" || strings.EqualFold(trimmed, "none") { continue }
        normalized = append(normalized, trimmed)
    }
    if len(normalized) == 0 { return nil }
    return normalized
}

// tasks/validate.go:150
func normalizeValidationDependencies(values []string) []string { /* identical body */ }
```

**Recommendation**: When parsing moves to `tasks` (F01), a single `normalizeDependencies` exported from `tasks` replaces both.

---

### F05 -- Duplicated metadata summary parser pattern (P1)

**Type**: DRY violation (structural duplication)
**Action**: **(C) Extraction**

`tasks/store.go:168-188` (`parseTaskMetaSummary`) and `reviews/store.go:398-418` (`parseRoundMetaSummary`) use the exact same regex-driven count extraction pattern against a `map[string]*int`:

```go
// tasks/store.go:168
func parseTaskMetaSummary(lines []string, meta *model.TaskMeta) error {
    counts := map[string]*int{"Total": &meta.Total, "Completed": &meta.Completed, "Pending": &meta.Pending}
    reCount := regexp.MustCompile(`^- (Total|Completed|Pending): (\d+)$`)
    for _, rawLine := range lines { /* ... identical loop body ... */ }
}

// reviews/store.go:398
func parseRoundMetaSummary(lines []string, meta *model.RoundMeta) error {
    counts := map[string]*int{"Total": &meta.Total, "Resolved": &meta.Resolved, "Unresolved": &meta.Unresolved}
    reCount := regexp.MustCompile(`^- (Total|Resolved|Unresolved): (\d+)$`)
    for _, rawLine := range lines { /* ... identical loop body ... */ }
}
```

**Recommendation**: Extract a generic `parseSummaryCounts(lines []string, counts map[string]*int) error` function into a shared utility, either in `frontmatter` or a new `internal/core/mdutil` package.

---

### F06 -- `prompt/common.go` is a 504-line god file with 4+ responsibilities (P1)

**Type**: Bloater / SRP violation
**Action**: **(A) File-level split** then **(B) Package-level split**

`internal/core/prompt/common.go` mixes:
1. **Task domain parsing** (lines 52-108, 134-175): `ParseTaskFile`, `ParseLegacyTaskFile`, `IsTaskCompleted`, `LooksLikeLegacyTaskFile`, `ExtractLegacyTaskBody`, `ExtractTaskNumber`
2. **Review domain parsing** (lines 272-369): `ParseReviewContext`, `ParseLegacyReviewContext`, `IsReviewResolved`, `LooksLikeLegacyReviewFile`, `ExtractLegacyReviewBody`, `ExtractIssueNumber`, `ParseReviewStatus`
3. **Prompt text building** (lines 38-50, 236-437): `Build`, `BuildSystemPromptAddendum`, `buildBatchHeader`, `buildBatchChecklist`, `FlattenAndSortIssues`
4. **File/path utilities** (lines 210-234, 459-504): `SafeFileName`, `NormalizeForPrompt`, `sanitizePath`, `extractXMLTag`, `extractLegacyStatus`

**Recommendation**: After F01 (moving parsing to domain packages), the remainder should be split into `prompt_build.go` (prompt construction), `prompt_utils.go` (file/path utilities), keeping `common.go` lean.

---

### F07 -- Regex compiled inside hot-path functions, not at package level (P1)

**Type**: Performance smell / Dispensable (repeated compilation)
**Action**: **(D) Inline fix**

Multiple `regexp.MustCompile` calls inside function bodies compile regexes on every invocation:

- `internal/core/prompt/common.go:136` -- `regexp.MustCompile("(?mi)^##\\s*status:")` called inside `LooksLikeLegacyTaskFile` (called per file)
- `internal/core/prompt/common.go:150` -- same regex inside `ExtractLegacyTaskBody` loop body
- `internal/core/prompt/common.go:174` -- `regexp.MustCompile("^task_(\\d+)\\.md$")` inside `ExtractTaskNumber` (called per file per sort comparison)
- `internal/core/prompt/common.go:178` -- `regexp.MustCompile("^issue_(\\d+)\\.md$")` inside `ExtractIssueNumber`
- `internal/core/prompt/common.go:323` -- duplicate of line 136 pattern in `LooksLikeLegacyReviewFile`
- `internal/core/prompt/common.go:337` -- duplicate of line 150 pattern in `ExtractLegacyReviewBody`
- `internal/core/prompt/common.go:452` -- `regexp.MustCompile(fmt.Sprintf(...))` inside `extractXMLTag`
- `internal/core/prompt/common.go:499` -- `regexp.MustCompile(...)` inside `extractLegacyStatus`
- `internal/core/plan/input.go:288,301,313` -- `regexp.MustCompile(...)` inside inference functions

**Recommendation**: Hoist all regexes to package-level `var` declarations (e.g., `var reTaskNumber = regexp.MustCompile(...)`) and reference them in functions.

---

### F08 -- `plan/input.go` uses `fmt.Println` for user messages (P1)

**Type**: Coding style violation / Coupling to stdout
**Action**: **(D) Inline fix**

`internal/core/plan/input.go:179,183,185,197` use `fmt.Println` for user-facing messages:
```go
fmt.Println("All task files are already completed. Nothing to do.")
fmt.Println("No task files found.")
fmt.Println("No review issue files found.")
fmt.Println("All review issues are already resolved. Nothing to do.")
```

Per project coding style: "Use `log/slog` for structured logging. Do not use `log.Printf` or `fmt.Println` for operational output." This also violates testability since output cannot be captured.

**Recommendation**: Replace with `slog.Info(...)` or propagate through the event bus.

---

### F09 -- `plan/input.go` readTaskEntries duplicates file walking from tasks/store.go countTasks (P1)

**Type**: DRY violation (structural duplication)
**Action**: **(C) Extraction**

`plan/input.go:216-262` (`readTaskEntries`) and `tasks/store.go:190-223` (`countTasks`) both:
1. Call `os.ReadDir(tasksDir)`
2. Filter for `*.md` files with `prompt.ExtractTaskNumber() != 0`
3. Sort by task number
4. Read each file, call `prompt.ParseTaskFile`, check completion status

The only difference is `readTaskEntries` builds `[]IssueEntry` while `countTasks` only counts.

**Recommendation**: Extract a shared `walkTaskFiles(dir string) ([]parsedTask, error)` in `tasks` and build both operations on top.

---

### F10 -- `plan` package high efferent coupling (P1)

**Type**: Coupling (efferent)
**Action**: **(B) Package-level split**

`plan/prepare.go` imports 10 internal packages:
```
agent, memory, model, preputil, prompt, reviews, tasks, journal, events
```

`plan/input.go` imports 4 more: `model, prompt, reviews, tasks`

This makes `plan` fragile -- any change to these 10+ packages risks breaking `plan`.

**Recommendation**: The `plan` package orchestrates too many concerns. After F01/F09, the parsing and file-walking logic should live in domain packages, reducing `plan` to only orchestration glue over the `model`, domain stores, and `prompt` builder.

---

### F11 -- `groupIssues` and `groupIssuesByCodeFile` near-duplicate (P1)

**Type**: DRY violation
**Action**: **(D) Inline fix**

`plan/input.go:278-284` (`groupIssues`) and `plan/prepare.go:429-440` (`groupIssuesByCodeFile`) both group `[]IssueEntry` by `CodeFile`. The second additionally returns sorted keys.

```go
// input.go:278
func groupIssues(entries []model.IssueEntry) map[string][]model.IssueEntry { ... }

// prepare.go:429
func groupIssuesByCodeFile(issues []model.IssueEntry) (map[string][]model.IssueEntry, []string) { ... }
```

**Recommendation**: Remove `groupIssues`, use `groupIssuesByCodeFile` everywhere (callers that don't need the keys can ignore the second return).

---

### F12 -- `ExtractLegacyTaskBody` and `ExtractLegacyReviewBody` structural duplication (P2)

**Type**: DRY violation
**Action**: **(C) Extraction**

`prompt/common.go:139-166` and `prompt/common.go:326-353` are structurally identical functions that strip XML context blocks from legacy content. They differ only in the open/close tag names (`<task_context>` vs `<review_context>`).

**Recommendation**: Extract a parameterized `extractLegacyBody(content, openTag, closeTag string) (string, error)` and call it from both.

---

### F13 -- `LooksLikeLegacyTaskFile` and `LooksLikeLegacyReviewFile` structural duplication (P2)

**Type**: DRY violation
**Action**: **(C) Extraction**

`prompt/common.go:134-137` and `prompt/common.go:321-324` are identical except for the XML tag name:
```go
func LooksLikeLegacyTaskFile(content string) bool {
    return strings.Contains(content, "<task_context>") ||
        regexp.MustCompile(`(?mi)^##\s*status:`).FindStringIndex(content) != nil
}
func LooksLikeLegacyReviewFile(content string) bool {
    return strings.Contains(content, "<review_context>") ||
        regexp.MustCompile(`(?mi)^##\s*status:`).FindStringIndex(content) != nil
}
```

**Recommendation**: Extract `looksLikeLegacyFile(content, xmlTag string) bool`.

---

### F14 -- `workspace/config.go` long validation chain without strategy pattern (P2)

**Type**: Bloater (438 lines), Conditional complexity
**Action**: **(A) File-level split**

`workspace/config.go` at 438 lines is manageable but mixes workspace discovery (lines 77-141), config loading (lines 143-167), config type definitions (lines 21-75), and a long validation chain (lines 169-438) with 12+ validator functions.

**Recommendation**: Split into `config_types.go` (struct definitions), `config_validate.go` (all validation functions), keeping `config.go` for `Resolve`/`Discover`/`LoadConfig`.

---

### F15 -- `prompt` error sentinels belong to domain packages (P2)

**Type**: Dependency inversion violation
**Action**: **(C) Extraction**

`prompt/common.go:21-24` defines sentinel errors:
```go
var (
    ErrLegacyTaskMetadata   = errors.New("legacy XML task metadata detected")
    ErrV1TaskMetadata       = errors.New("v1 task front matter detected")
    ErrLegacyReviewMetadata = errors.New("legacy XML review metadata detected")
)
```

These are checked via `errors.Is()` in `tasks/store.go:226`, `reviews/store.go:421`, `plan/input.go:326-333`, `migrate.go:237`. Domain error sentinels should live in their respective domain packages.

**Recommendation**: Move `ErrLegacyTaskMetadata` and `ErrV1TaskMetadata` to `tasks`. Move `ErrLegacyReviewMetadata` to `reviews`. (Part of F01.)

---

### F16 -- `reviews/store.go` TaskDirectoryForWorkspace is a pass-through (P2)

**Type**: Middle Man / Dispensable
**Action**: **(D) Inline fix**

`reviews/store.go:31-33`:
```go
func TaskDirectoryForWorkspace(workspaceRoot, name string) string {
    return model.TaskDirectoryForWorkspace(workspaceRoot, name)
}
```

This is a pure delegation to `model`. Callers could use `model.TaskDirectoryForWorkspace` directly.

**Recommendation**: Remove the wrapper; update `plan/input.go:108` to call `model.TaskDirectoryForWorkspace` directly.

---

### F17 -- `reviews` and `tasks` store meta format/parse duplication (P2)

**Type**: DRY violation (structural pattern)
**Action**: **(C) Extraction**

Both `tasks/store.go` and `reviews/store.go` implement the same meta lifecycle pattern:
1. `formatXxxMeta()` -- marshal frontmatter struct + summary body
2. `parseXxxMeta()` -- parse frontmatter struct + parse summary counts
3. `ReadXxxMeta()` -- read file + parse
4. `WriteXxxMeta()` -- format + write file
5. `RefreshXxxMeta()` -- read existing + recount + write

The format/parse/read/write/refresh cycle is identical in structure. Both use `frontmatter.Format` / `frontmatter.Parse` with a local struct, plus a regex-based summary parser.

**Recommendation**: Consider a lightweight generic `MetaStore[T]` helper or at minimum extract the regex summary parsing (F05).

---

### F18 -- `prompt.BatchParams` carries `cfg` fields individually instead of accepting config (P2)

**Type**: Data Clump / Long Parameter Object
**Action**: **(D) Inline fix**

`prompt/common.go:26-36` (`BatchParams`) mirrors fields from `model.RuntimeConfig`:
```go
type BatchParams struct {
    Name, Provider, PR, ReviewsDir string
    Round int
    BatchGroups map[string][]model.IssueEntry
    AutoCommit bool
    Mode model.ExecutionMode
    Memory *WorkflowMemoryContext
}
```

`plan/prepare.go:217-226` manually copies each field from `cfg` into `BatchParams`.

**Recommendation**: Either embed a `model.RuntimeConfig` subset interface or accept `*model.RuntimeConfig` directly in `prompt.Build()`.

---

### F19 -- `buildBatchJob` is a 62-line function with mixed concerns (P2)

**Type**: Bloater (long function)
**Action**: **(D) Inline fix**

`plan/prepare.go:204-266` (`buildBatchJob`) handles grouping, naming, task parsing, memory preparation, prompt building, and artifact writing in a single function.

**Recommendation**: Extract subroutines: `resolveBatchTaskData(cfg, batchIssues) (TaskEntry, *WorkflowMemoryContext, error)` and keep `buildBatchJob` as the compositor.

---

### F20 -- `templates.go` `mustReadTemplate` silently returns empty string on error (P2)

**Type**: Error swallowing
**Action**: **(D) Inline fix**

`prompt/templates.go:30-35`:
```go
func mustReadTemplate(name string) string {
    content, err := templateFS.ReadFile("prompts/" + name)
    if err != nil {
        return ""
    }
    return string(content)
}
```

A "must" function that silently swallows errors is misleading. Since templates are embedded at compile time, a missing template is a programmer error.

**Recommendation**: Either `panic` (it's a compile-time-embedded resource) or rename to `readTemplate` and propagate the error.

---

### F21 -- `preputil` package is over-packaged for a single function (P3)

**Type**: Lazy Element
**Action**: **(C) Extraction / inline**

`internal/core/preputil/journal.go` contains only `ClosePreparationJournal` (40 lines). A single-function package adds navigational overhead.

**Recommendation**: Move into `plan` package (its only non-test consumer is `plan/prepare.go:38`) or into `model` alongside `SolvePreparation`.

---

### F22 -- `memory` package `countLines` utility could live in a shared string util (P3)

**Type**: Minor cohesion concern
**Action**: **(D) Inline fix** (low priority)

`memory/store.go:116-126` (`countLines`) is a generic string utility. Currently only used within `memory`, so this is low priority unless other packages need similar logic.

**Recommendation**: Keep in place unless a shared `strutil` package emerges from other refactoring.

---

### F23 -- Unused reasoning effort constants in `templates.go` (P3)

**Type**: Potential dead code / Dispensable
**Action**: **(D) Inline fix**

`prompt/templates.go:11-15`:
```go
const (
    reasoningEffortLow    = "low"
    reasoningEffortMedium = "medium"
    reasoningEffortHigh   = "high"
    reasoningEffortXHigh  = "xhigh"
)
```

These constants duplicate the string literals already used in `workspace/config.go:303` (`"low", "medium", "high", "xhigh"`) and the switch in `ClaudeReasoningPrompt` could use string literals directly, or these constants should be shared.

**Recommendation**: Either share constants from a single source or inline the strings.

---

### F24 -- `resolveReviewInputs` is a 57-line function with nested conditionals (P3)

**Type**: Bloater / Conditional complexity
**Action**: **(D) Inline fix**

`plan/input.go:100-156` has 3 levels of conditional nesting and handles multiple fallback strategies.

**Recommendation**: Extract `resolveReviewDirFromName(cfg) (string, error)` to simplify the main function.

---

## Dependency Graph (Afferent/Efferent)

```
Package            Afferent (dependents)  Efferent (dependencies)
-------            --------------------   -----------------------
frontmatter        5 (prompt,tasks,       0 internal
                    reviews,migrate,
                    setup/catalog)
prompt             8 (plan,tasks,         2 (frontmatter,model)
                    reviews,run,
                    migrate)
tasks              7 (plan,cli,           3 (frontmatter,model,prompt)
                    workspace,run,
                    migrate,sync,
                    archive)
reviews            6 (plan,run,migrate,   4 (frontmatter,model,prompt,
                    fetch,archive,cli)     provider)
memory             1 (plan)               1 (model -- implicit via paths)
workspace          4 (cli,kernel,         4 (agent,model,providers,tasks)
                    migrate)
plan               3 (kernel,api,         10 (agent,memory,model,preputil,
                    run/integration)        prompt,reviews,tasks,journal,
                                            events)
preputil           1 (plan)               1 (model)
```

**Highest risk**: `prompt` (8 dependents, high afferent coupling -- dangerous to change). `plan` (10 dependencies, high efferent coupling -- fragile).

---

## Recommended Refactoring Order

### Phase 1: Quick Wins (trivial effort, high impact)

| # | Finding | Effort | Impact |
|---|---------|--------|--------|
| 1 | F02 + F03: Consolidate duplicated error wrappers | trivial | eliminates 2 copy-paste sites |
| 2 | F07: Hoist regexes to package-level vars | trivial | performance + readability |
| 3 | F08: Replace `fmt.Println` with `slog.Info` | trivial | coding style compliance |
| 4 | F11: Merge `groupIssues` into `groupIssuesByCodeFile` | trivial | eliminates duplication |
| 5 | F20: Fix `mustReadTemplate` error handling | trivial | correctness |

### Phase 2: Domain Restructuring (moderate effort, critical impact)

| # | Finding | Effort | Impact |
|---|---------|--------|--------|
| 6 | F01 + F15: Move parsing functions and sentinels from `prompt` to `tasks`/`reviews` | moderate | fixes architectural inversion, reduces coupling |
| 7 | F06: Split `prompt/common.go` (after F01, most parsing is gone) | moderate | SRP compliance |
| 8 | F04: Consolidate `normalizeDependencies` | trivial (part of F01) | DRY |
| 9 | F09: Extract shared task file walker | moderate | DRY, single source of truth |

### Phase 3: Structural Cleanup (moderate effort, medium impact)

| # | Finding | Effort | Impact |
|---|---------|--------|--------|
| 10 | F05 + F17: Extract generic meta summary parser | moderate | DRY across stores |
| 11 | F12 + F13: Parameterize legacy body/detect functions | trivial | DRY |
| 12 | F14: Split `workspace/config.go` into files | trivial | readability |
| 13 | F16: Remove `reviews.TaskDirectoryForWorkspace` wrapper | trivial | remove middle man |
| 14 | F21: Fold `preputil` into `plan` | trivial | reduce package count |

### Phase 4: Polish (low effort, low impact)

| # | Finding | Effort | Impact |
|---|---------|--------|--------|
| 15 | F18: Simplify `BatchParams` construction | trivial | cleaner API |
| 16 | F19: Extract subroutine from `buildBatchJob` | trivial | readability |
| 17 | F23: Share or inline reasoning effort constants | trivial | DRY |
| 18 | F24: Simplify `resolveReviewInputs` | trivial | readability |
