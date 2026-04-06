# Refactoring Analysis: Group 5 -- Provider, Setup & Public Packages

**Date**: 2026-04-06
**Analyst**: Claude Opus 4.6
**Scope**: `internal/core/provider`, `internal/core/providers`, `internal/setup`, `internal/charmtheme`, `internal/version`, `pkg/compozy/events`, `pkg/compozy/runs`

---

## Executive Summary

| Severity | Count |
|----------|-------|
| P0       | 2     |
| P1       | 6     |
| P2       | 9     |
| P3       | 5     |
| **Total**| **22**|

**Top 5 highest-impact opportunities:**

1. **P0** -- Massive code duplication between `internal/core/model/content.go` (458 lines) and `pkg/compozy/events/kinds/session.go` (449 lines): near-identical type hierarchies for `ContentBlock`, `SessionUpdate`, `Usage`, all block types, and all decode/encode machinery. The internal variant uses camelCase JSON tags; the public variant uses snake_case. This is a maintenance trap.
2. **P0** -- `internal/version` package is dead code at the Go level -- it is only populated via linker flags (`-ldflags -X`) and never imported by any `.go` file in the repository.
3. **P1** -- `agents.go` is a 435-line data slab of 35+ agent specifications with repetitive closures, making every agent addition a shotgun surgery candidate.
4. **P1** -- `watch.go` in `pkg/compozy/runs` is 586 lines with deeply nested helper functions passing 7-9 parameters, indicating the file should be split into focused watcher components.
5. **P1** -- Shutdown payload types in `pkg/compozy/events/kinds/shutdown.go` contain three near-identical structs differing by a single boolean field.

---

## Package Summaries

### `internal/core/provider` (provider.go, registry.go)
Clean interface + registry pattern. `Provider` interface defines `FetchReviews` and `ResolveIssues`. `Registry` maps names to implementations. Small, cohesive, well-separated. 3 files, ~66 lines total production code.

### `internal/core/provider/coderabbit` (coderabbit.go)
391-line concrete implementation of the `Provider` interface for CodeRabbit. Uses `gh` CLI via a `CommandRunner` abstraction. Contains REST pagination, GraphQL queries, JSON response types, and helper functions for provider ref encoding.

### `internal/core/providers` (defaults.go)
13-line factory that constructs a `DefaultRegistry()` pre-populated with the `coderabbit` provider. Acts as the composition root for the provider subsystem.

### `internal/setup` (agents.go, install.go, verify.go, bundle.go, catalog.go, types.go, runtime_agents.go)
Skill installation pipeline for 35+ AI coding agents. Handles detection, path resolution, install (copy/symlink), verification, and drift detection. ~1800+ lines across 7 production files.

### `internal/charmtheme` (theme.go)
43-line design token file defining Lipgloss color constants and a border style. Pure data, no logic.

### `internal/version` (version.go)
13-line build metadata placeholder populated only via `-ldflags -X`.

### `pkg/compozy/events` (event.go, bus.go, doc.go, kinds/*.go)
Public event system: envelope struct, generic fan-out bus, and 10+ kind-specific payload files. The `kinds/session.go` file (449 lines) is the largest, containing content block types, session update structures, encode/decode machinery, and type-safe block accessors.

### `pkg/compozy/runs` (run.go, scanner.go, tail.go, watch.go, summary.go, doc.go)
Public run reader library. `run.go` (451 lines) handles metadata loading, path resolution, event replay. `watch.go` (586 lines) implements workspace-level fsnotify watcher. `tail.go` (187 lines) implements live event tailing via `nxadm/tail`. `summary.go` (125 lines) handles listing and filtering.

---

## Detailed Findings

### F01: Massive DRY violation -- `model/content.go` vs `events/kinds/session.go`

**Severity**: P0
**Smell**: Duplicated Code (entire type hierarchies)
**Action**: (C) Extraction -- create a shared content-block package or make one the canonical source

**Description**: `internal/core/model/content.go` (458 lines) and `pkg/compozy/events/kinds/session.go` (449 lines) contain structurally identical code:

- `ContentBlockType` enum with identical 6 variants (`text`, `tool_use`, `tool_result`, `diff`, `terminal_output`, `image`)
- `ContentBlock` struct with identical `MarshalJSON`/`UnmarshalJSON`
- 6 typed block structs (`TextBlock`, `ToolUseBlock`, `ToolResultBlock`, `DiffBlock`, `TerminalOutputBlock`, `ImageBlock`)
- `SessionUpdate`, `SessionStatus`, `SessionUpdateKind`, `ToolCallState` enums
- `SessionPlanEntry`, `SessionAvailableCommand` structs
- `Usage` struct with identical `Add()` and `Total()` methods
- `NewContentBlock()` factory with identical `reflect`-based nil check
- 6 `decode*Block()` + 6 `ensure*Block()` + 6 `normalizeContentBlock()` methods
- `contentBlockNormalizer` interface

The only difference is JSON tag casing: `model` uses camelCase (`toolCallId`, `exitCode`), `kinds` uses snake_case (`tool_call_id`, `exit_code`). This means the duplication is intentional to support two serialization formats, but the logic (decode, validate, ensure, normalize) is identical.

**Files**:
- `internal/core/model/content.go:1-458`
- `pkg/compozy/events/kinds/session.go:1-449`

**Recommendation**: Extract shared logic into a generic content-block engine parameterized by JSON tag strategy. The concrete types could live in their respective packages but share the generic decode/encode/validate machinery. Alternatively, use a single canonical set of types with a conversion/remapping layer for the alternate JSON format.

---

### F02: Dead code -- `internal/version` package

**Severity**: P0
**Smell**: Dead Code (Dispensable)
**Action**: (D) Inline fix -- either wire the import or document the linker-only usage

**Description**: `internal/version/version.go` defines `Version`, `Commit`, `Date` variables and a `String()` function. These are populated solely via `-ldflags -X` in the Makefile and `.goreleaser.yml`. No Go source file imports this package. The `String()` function is never called.

**File**: `internal/version/version.go:1-13`

**Recommendation**: Either (a) add an import in `cmd/compozy/main.go` or the root `command` package to actually use `version.String()` for `--version` output, or (b) document that this package exists exclusively for linker injection and the `String()` function is dead. If the CLI already has version output via another mechanism, the `String()` function should be removed.

---

### F03: Agent spec data slab -- `internal/setup/agents.go`

**Severity**: P1
**Smell**: Long Function / Data Clump / Shotgun Surgery
**Action**: (A) File-level split -- extract agent data to a declarative data file or table

**Description**: `agents.go` (435 lines) defines 35+ agent specifications in a `var agentSpecs = []agentSpec{...}` literal spanning lines 172-404. Each entry is a `universalAgent()` or `specificAgent()` call with repetitive closure patterns. Adding a new agent requires:
1. Adding a new entry to the slab (lines 172-404)
2. Potentially adding to `runtimeIDEAgentNames` in `runtime_agents.go`
3. Potentially adding to `agentAliases` in `agents.go`

The closures for `globalDir` and `detect` are nearly identical for most agents (just changing a path segment). This is a textbook Data Clump.

**File**: `internal/setup/agents.go:172-404`

**Recommendation**: Replace the closure-heavy slab with a declarative table (struct slice) that encodes just the varying data (name, display name, project dir pattern, home dir pattern, detection path). A single `resolveGlobalDir` and `resolveDetected` function can interpret the patterns.

---

### F04: `watch.go` is a 586-line monolith with excessive parameter threading

**Severity**: P1
**Smell**: Long Function / Long Parameter List
**Action**: (A) File-level split or (B) Package-level split into watcher component

**Description**: `pkg/compozy/runs/watch.go` is 586 lines. Several functions pass 7-9 parameters:

- `processWorkspaceWatcherEvent` (line 135): 8 params
- `handleInfrastructureEvent` (line 175): 9 params
- `handleWorkspaceEvent` (line 340): 8 params
- `syncWorkspaceRuns` (line 246): 7 params
- `seedWorkspaceWatcher` (line 213): 5 params

The `workspaceWatchState` struct (line 42) was introduced to group some state, but the helper functions still destructure it into individual map parameters. This is Feature Envy -- the functions should operate on the state struct directly.

**Files**:
- `pkg/compozy/runs/watch.go:135-173` (processWorkspaceWatcherEvent)
- `pkg/compozy/runs/watch.go:175-200` (handleInfrastructureEvent)
- `pkg/compozy/runs/watch.go:340-380` (handleWorkspaceEvent)

**Recommendation**: Refactor helper functions to accept `*workspaceWatchState` instead of individual maps. Group the watcher, runsDir, and workspaceRoot into a receiver type. This would reduce most functions to 3-4 params.

---

### F05: Shutdown payload duplication in `events/kinds/shutdown.go`

**Severity**: P1
**Smell**: Duplicated Code / Data Clumps
**Action**: (D) Inline fix -- embed a shared base struct

**Description**: Three shutdown payload structs share identical fields:

```go
type ShutdownRequestedPayload struct {
    Source      string    `json:"source,omitempty"`
    RequestedAt time.Time `json:"requested_at,omitzero"`
    DeadlineAt  time.Time `json:"deadline_at,omitzero"`
}
type ShutdownDrainingPayload struct {
    Source      string    `json:"source,omitempty"`
    RequestedAt time.Time `json:"requested_at,omitzero"`
    DeadlineAt  time.Time `json:"deadline_at,omitzero"`
}
type ShutdownTerminatedPayload struct {
    Source      string    `json:"source,omitempty"`
    RequestedAt time.Time `json:"requested_at,omitzero"`
    DeadlineAt  time.Time `json:"deadline_at,omitzero"`
    Forced      bool      `json:"forced,omitempty"`
}
```

**File**: `pkg/compozy/events/kinds/shutdown.go:6-26`

**Recommendation**: Extract a `ShutdownBase` embedded struct. `ShutdownTerminatedPayload` embeds it and adds the `Forced` field. The other two become type aliases or thin wrappers. This is a minor improvement but prevents drift.

---

### F06: Job payload structs repeat `Index`, `Attempt`, `MaxAttempts` across 7 types

**Severity**: P1
**Smell**: Data Clumps / Primitive Obsession
**Action**: (D) Inline fix -- extract `JobAttemptInfo` embedded struct

**Description**: In `pkg/compozy/events/kinds/job.go`, seven payload structs repeat the same field triplet:

```go
Index       int `json:"index"`
Attempt     int `json:"attempt,omitempty"`
MaxAttempts int `json:"max_attempts,omitempty"`
```

Repeated in: `JobQueuedPayload`, `JobStartedPayload`, `JobAttemptStartedPayload`, `JobAttemptFinishedPayload`, `JobRetryScheduledPayload`, `JobCompletedPayload`, `JobFailedPayload`, `JobCancelledPayload`.

**File**: `pkg/compozy/events/kinds/job.go:1-77`

**Recommendation**: Extract a `JobAttemptInfo` struct and embed it in each payload. This reduces boilerplate and ensures field names stay consistent.

---

### F07: Provider payload structs repeat `CallID`, `Provider`, `Endpoint`, `Method`

**Severity**: P1
**Smell**: Data Clumps
**Action**: (D) Inline fix -- extract `ProviderCallInfo` embedded struct

**Description**: In `pkg/compozy/events/kinds/provider.go`, three payload structs share:

```go
CallID   string `json:"call_id"`
Provider string `json:"provider"`
Endpoint string `json:"endpoint,omitempty"`
Method   string `json:"method,omitempty"`
```

**File**: `pkg/compozy/events/kinds/provider.go:1-34`

**Recommendation**: Extract `ProviderCallInfo` and embed it.

---

### F08: `run.go` in `pkg/compozy/runs` mixes 3 concerns in 451 lines

**Severity**: P1
**Smell**: Large File / Divergent Change
**Action**: (A) File-level split

**Description**: `run.go` contains three distinct responsibilities:
1. **Run loading and metadata** (lines 96-144, 226-293): `Open`, `loadRun`, path resolution, summary normalization
2. **Event replay** (lines 155-224): `Replay` iterator with partial-line tolerance
3. **Status derivation** (lines 295-451): `deriveRunState`, `loadResultStatus`, `bestEffortRunStateFromEvents`, `validateSchemaVersion`, `normalizeStatus`, `isTerminalStatus`, utility helpers

These change for different reasons: metadata format changes affect (1), event schema changes affect (2) and (3), status semantics changes affect (3).

**File**: `pkg/compozy/runs/run.go:1-451`

**Recommendation**: Split into `run.go` (loading, metadata, Open), `replay.go` (Replay method, event decoding), `status.go` (status derivation, normalization, terminal checks).

---

### F09: `provider` vs `providers` package split is sensible but naming is confusing

**Severity**: P2
**Smell**: Potential architectural confusion
**Action**: (D) Inline fix -- rename `providers` to `providerdefaults` or merge

**Description**: `internal/core/provider` (interface + registry) and `internal/core/providers` (factory) are separate packages. The split follows dependency inversion correctly: `providers` imports `provider` and `coderabbit`, not the other way around. However, the naming (`provider` vs `providers`) is confusingly similar.

**Files**:
- `internal/core/provider/` (3 files, ~66 lines)
- `internal/core/providers/defaults.go` (13 lines)

**Recommendation**: Either rename `providers` to `providerdefaults` or `providersetup` for clarity, or consider inlining `DefaultRegistry()` into the call site (it is only used in 3 files: `internal/core/run/execution.go`, `internal/core/workspace/config.go`, `internal/core/fetch.go`).

---

### F10: `coderabbit.go` GraphQL query strings are embedded inline

**Severity**: P2
**Smell**: Long Function / Magic Strings
**Action**: (D) Inline fix -- extract to package-level constants

**Description**: `fetchReviewThreads` (lines 201-265) and `resolveThread` (lines 267-286) embed multi-line GraphQL query strings inline. The `fetchReviewThreads` function itself is 64 lines of mixed GraphQL + pagination logic.

**File**: `internal/core/provider/coderabbit/coderabbit.go:207-229` (query literal), `268-274` (mutation literal)

**Recommendation**: Extract GraphQL strings to package-level `const` blocks. Consider splitting `fetchReviewThreads` into the query construction and the pagination loop.

---

### F11: `install.go` `installPreviewItem` has a long switch with duplicated copy logic

**Severity**: P2
**Smell**: Duplicated Code within function
**Action**: (D) Inline fix

**Description**: In `installPreviewItem` (lines 140-198), the `InstallModeSymlink` branch (lines 155-195) contains a fallback that duplicates the entire `InstallModeCopy` branch when symlinking fails. The `cleanAndCreateDirectory` + `copySkillDirectory` sequence appears three times.

**File**: `internal/setup/install.go:140-198`

**Recommendation**: Extract the copy operation into a helper `installViaCopy(bundle, item) (*SuccessItem, *FailureItem)` and call it from both the `Copy` branch and the symlink fallback.

---

### F12: `SelectSkills` and `SelectAgents` are structurally identical

**Severity**: P2
**Smell**: Duplicated Code
**Action**: (C) Extraction -- extract generic select-by-name function

**Description**: `SelectSkills` (install.go:17-52) and `SelectAgents` (agents.go:56-95) follow the exact same pattern:
1. Build index map from name to entity
2. Iterate requested names
3. Collect invalid names
4. Deduplicate
5. Sort result

The only difference is the entity type (`Skill` vs `Agent`) and the alias handling in `SelectAgents`.

**Files**:
- `internal/setup/install.go:17-52`
- `internal/setup/agents.go:56-95`

**Recommendation**: Extract a generic `selectByName[T any]()` function parameterized by the key extraction function and an optional alias map.

---

### F13: `VerifyResult` convenience methods use identical filtering pattern

**Severity**: P2
**Smell**: Duplicated Code
**Action**: (D) Inline fix

**Description**: `MissingSkillNames()` and `DriftedSkillNames()` in `types.go` (lines 114-137) are identical except for the `VerifyState` constant. Similarly, `HasMissing()` and `HasDrift()` (lines 139-157) are identical except for the state check.

**File**: `internal/setup/types.go:114-157`

**Recommendation**: Extract `skillNamesByState(state VerifyState) []string` and `hasState(state VerifyState) bool` helper methods, then express the public methods as one-liners.

---

### F14: `session.go` in `events/kinds` mixes content block machinery with session payloads

**Severity**: P2
**Smell**: Divergent Change / SRP violation
**Action**: (A) File-level split

**Description**: `kinds/session.go` (449 lines) contains two distinct responsibilities:
1. **Content block types and codec** (lines 9-80, 81-450): `ContentBlockType`, all block structs, `ContentBlock` marshal/unmarshal, decode/ensure functions, normalizer interface
2. **Session event payloads** (lines 138-191): `SessionUpdate`, `SessionStartedPayload`, `SessionUpdatePayload`, `SessionCompletedPayload`, `SessionFailedPayload`

The content block machinery is reusable beyond sessions and is already used by the tool_call payloads.

**File**: `pkg/compozy/events/kinds/session.go:1-449`

**Recommendation**: Split into `content_block.go` (types, codec, normalizers) and `session.go` (session-specific payloads). This makes the content block types discoverable for other event kinds.

---

### F15: `runtime_agents.go` has a hardcoded IDE-to-agent mapping table

**Severity**: P2
**Smell**: Shotgun Surgery potential
**Action**: (D) Inline fix -- derive from `agentSpecs` or `agentAliases`

**Description**: `runtime_agents.go` maintains `runtimeIDEAgentNames` (lines 8-16), a separate mapping from runtime IDE names to agent names. This is a third mapping (alongside `agentSpecs` and `agentAliases`) that must be kept in sync manually.

**File**: `internal/setup/runtime_agents.go:8-16`

**Recommendation**: Consider whether this mapping can be derived from `agentSpecs` by adding an `ideNames []string` field to `agentSpec`, eliminating the separate maintenance point.

---

### F16: `coderabbit.go` response types are private but tightly coupled to GitHub API

**Severity**: P2
**Smell**: Insider Trading / Tight coupling to external API
**Action**: (D) Inline fix -- extract response types to a separate file

**Description**: Lines 343-391 define 5 response structs (`pullRequestComment`, `reviewThreadsResponse`, `reviewThreadConnection`, `reviewThread`, `reviewThreadComment`) that model GitHub's REST/GraphQL API responses. These are mixed in with the business logic.

**File**: `internal/core/provider/coderabbit/coderabbit.go:343-391`

**Recommendation**: Move response types to a `types.go` file within the `coderabbit` package for better organization. This is a minor file-level reorganization.

---

### F17: `charmtheme` exports raw Lipgloss values without abstraction

**Severity**: P3
**Smell**: Primitive Obsession (potential)
**Action**: (D) Inline fix -- no immediate action needed

**Description**: `charmtheme/theme.go` exports 15+ individual `lipgloss.Color` variables and a `lipgloss.Border` struct. The package has only 2 importers (`internal/core/run/ui_styles.go` and `internal/cli/theme.go`). This is acceptable for a small design-token file, but if the theme grows, consider grouping into a `Theme` struct.

**File**: `internal/charmtheme/theme.go:1-43`

**Recommendation**: No action needed now. Monitor for growth; if theme variants are needed, convert to a `Theme` struct with a `Default()` constructor.

---

### F18: `bus.go` Subscribe double-checks `closed` flag

**Severity**: P3
**Smell**: Minor defensive redundancy
**Action**: (D) Inline fix -- acceptable pattern

**Description**: `Subscribe()` (bus.go:47-73) checks `b.closed.Load()` before acquiring the lock and again after. This is a standard double-checked locking pattern and is correct, but the pre-lock check is an optimization that adds cognitive overhead for minimal benefit given the bus is typically not closed during active subscription.

**File**: `pkg/compozy/events/bus.go:47-62`

**Recommendation**: This is defensible. No action required unless readability is a concern.

---

### F19: `bundle.go` exposes `bundledSkillsRoot()` only for tests

**Severity**: P3
**Smell**: Comments as Deodorant / Speculative Generality
**Action**: (D) Inline fix

**Description**: `bundledSkillsRoot()` (bundle.go:33-35) is documented as "returns the embedded skill filesystem for tests" but is unexported and only used indirectly. The function is trivial (returns `skills.FS, nil`).

**File**: `internal/setup/bundle.go:33-35`

**Recommendation**: If this is only used in tests, move it to a `_test.go` file or remove it entirely.

---

### F20: `providerRefValue` and `buildProviderRef` use ad-hoc encoding

**Severity**: P3
**Smell**: Primitive Obsession
**Action**: (D) Inline fix

**Description**: `coderabbit.go` uses a comma-separated `key:value` format for `ProviderRef` strings (lines 305-327). This ad-hoc encoding could break on values containing `:` or `,`.

**File**: `internal/core/provider/coderabbit/coderabbit.go:305-327`

**Recommendation**: Consider using a structured encoding (URL query params, JSON) or at minimum document the format invariants. Low priority since GitHub IDs are unlikely to contain these characters.

---

### F21: `watch.go` uses `time.NewTicker` polling loop in `refreshWorkspaceRunEventually`

**Severity**: P3
**Smell**: Polling anti-pattern
**Action**: (D) Inline fix

**Description**: `refreshWorkspaceRunEventually` (watch.go:426-462) polls every 10ms with a 250ms timeout to wait for a run's metadata to become ready. This is a workaround for the race between directory creation and file write.

**File**: `pkg/compozy/runs/watch.go:426-462`

**Recommendation**: This is a pragmatic solution to an fsnotify race condition. Consider documenting the rationale inline. The polling interval (10ms) could be bumped to 25ms without noticeable impact.

---

### F22: `isTransientRunLoadError` string-matches error messages

**Severity**: P2
**Smell**: Fragile error matching
**Action**: (D) Inline fix

**Description**: `isTransientRunLoadError` (watch.go:475-482) checks `strings.Contains(err.Error(), "unexpected end of JSON input")` alongside the proper `errors.As` check for `*json.SyntaxError`. The string matching is fragile.

**File**: `pkg/compozy/runs/watch.go:475-482`

**Recommendation**: Check if Go's `json` package uses a typed error for "unexpected end of JSON input" (it is `*json.SyntaxError` in some cases). If not, this is an acceptable workaround but should be documented.

---

## Coupling Analysis

### Afferent Coupling (packages depended ON by many others -- risky to change)

| Package | Importers (non-test) |
|---------|---------------------|
| `pkg/compozy/events` | 13+ files across `internal/core/run`, `pkg/compozy/runs`, `internal/core/kernel`, `internal/core/plan`, `internal/cli` |
| `pkg/compozy/events/kinds` | 13+ files across `internal/core/run`, `internal/core/model` (duplicate), scripts |
| `internal/core/provider` | 5 files: `coderabbit.go`, `defaults.go`, `execution.go`, `store.go`, `plan/prepare_test.go` |

### Efferent Coupling (packages that depend ON many others -- fragile)

| Package | Dependencies |
|---------|-------------|
| `internal/setup` | `internal/core/frontmatter`, `github.com/compozy/compozy/skills` (embedded FS) |
| `pkg/compozy/runs` | `pkg/compozy/events`, `github.com/nxadm/tail`, `github.com/fsnotify/fsnotify` |
| `internal/core/providers` | `internal/core/provider`, `internal/core/provider/coderabbit` |

### Circular Dependencies

No circular dependencies detected in the scoped packages.

### Dependency Inversion Assessment

The `provider` package correctly defines an interface, with `coderabbit` as a concrete implementation and `providers/defaults.go` as the composition root. This follows DIP well.

The `events` and `events/kinds` packages are pure data types with no external dependencies (besides `encoding/json` and `time`), which is excellent for a public API package.

---

## SOLID Analysis

### SRP Violations
- **`kinds/session.go`**: Mixes content block codec with session payloads (F14)
- **`runs/run.go`**: Mixes metadata loading, event replay, and status derivation (F08)
- **`runs/watch.go`**: Acceptably large but the parameter threading suggests the state management responsibility should be consolidated (F04)

### OCP Violations
- **Agent specifications** (F03): Adding a new agent requires modifying the `agentSpecs` slab rather than registering via a plugin/extension mechanism
- **Content block types**: Adding a new block type requires editing both `model/content.go` and `kinds/session.go` (F01) -- double the modification

### ISP Assessment
- **`provider.Provider` interface** is well-scoped (3 methods). No ISP concerns.
- No oversized interfaces detected in the scoped packages.

### DIP Assessment
- `internal/core/providers` correctly depends on abstractions (`provider.Provider` interface) while providing concrete implementations
- `internal/setup` depends on `skills.FS` (concrete embedded filesystem) in `bundle.go`, but this is acceptable for an embed boundary

---

## Recommended Refactoring Order

### Quick Wins (trivial effort, immediate value)
1. **F05**: Extract `ShutdownBase` in shutdown.go -- 5 minutes
2. **F13**: Extract `skillNamesByState` in types.go -- 5 minutes
3. **F19**: Move `bundledSkillsRoot` to test file -- 2 minutes
4. **F16**: Extract coderabbit response types to types.go -- 10 minutes

### High Impact (moderate effort)
5. **F02**: Wire `internal/version` import or document linker-only usage -- 15 minutes
6. **F08**: Split `run.go` into run/replay/status -- 30 minutes
7. **F14**: Split `session.go` into content_block/session -- 30 minutes
8. **F04**: Refactor `watch.go` to use receiver-based state -- 45 minutes
9. **F11**: Extract `installViaCopy` helper in install.go -- 15 minutes
10. **F12**: Extract generic `selectByName` function -- 20 minutes

### Strategic (significant effort, high long-term value)
11. **F01**: Unify `model/content.go` and `kinds/session.go` -- 2-4 hours (requires careful migration of all importers)
12. **F03**: Replace agent spec closures with declarative table -- 1-2 hours
13. **F15**: Derive `runtimeIDEAgentNames` from `agentSpecs` -- 30 minutes
