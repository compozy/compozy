# First-Class Extension Provider Registration for Compozy

## Problem Statement

Compozy's extension system (introduced in commit `a1aa781`) has a mature subprocess-based JSON-RPC 2.0 plugin architecture with 28+ lifecycle hooks, capability-based security, and three-level discovery (bundled, user, workspace). However, **there is no dedicated, first-class mechanism for extensions to register new ACP drivers (IDE providers), review providers, or model providers**. What exists today is a patchwork of separate pieces with significant gaps that prevent extensions from truly adding new providers.

The goal is to design and implement a cohesive, first-class provider registration system so that extensions can add new ACP drivers, review providers, and (future) model providers -- with the same quality and discoverability as built-in ones.

---

## Current System Context

### Extension Architecture Overview

Extensions are subprocess-based plugins communicating via JSON-RPC 2.0 over stdin/stdout. Each extension declares an `extension.toml` manifest with:

- `[extension]` -- name, version, description
- `[subprocess]` -- command, args, env (for executable extensions)
- `[security]` -- capabilities (19 total)
- `[[hooks]]` -- lifecycle hook subscriptions (28 hook points)
- `[resources]` -- skill packs (`skills = ["path/to/skill"]`)
- `[providers]` -- IDE, review, and model provider declarations

Discovery scans three sources: `internal/core/extension/builtin/` (bundled, currently empty), `~/.compozy/extensions/` (user), `.compozy/extensions/` (workspace). Precedence: workspace > user > bundled.

The Host API exposes 6 namespaces to extensions via JSON-RPC:

- `host.events` -- subscribe/publish
- `host.tasks` -- list/get/create
- `host.runs` -- start child runs
- `host.artifacts` -- read/write files
- `host.prompts` -- render templates
- `host.memory` -- read/write workflow memory

**There is NO `host.providers` namespace.**

---

## Provider Type 1: ACP Drivers (IDE Providers)

### Current Implementation

8 drivers hardcoded in `internal/core/agent/registry_specs.go` (lines 84-250):

```go
type Spec struct {
    ID                 string
    DisplayName        string
    SetupAgentName     string
    DefaultModel       string
    Command            string
    FixedArgs          []string
    ProbeArgs          []string
    Fallbacks          []Launcher    // NOT supported by overlay
    SupportsAddDirs    bool
    UsesBootstrapModel bool
    EnvVars            map[string]string
    DocsURL            string
    InstallHint        string
    FullAccessModeID   string
    BootstrapArgs      func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string  // NOT supported by overlay
}
```

Built-in drivers: Claude, Codex, Droid, Cursor, OpenCode, Pi, Gemini, Copilot.

### Extension Overlay Mechanism (Partial)

An extension can declare in `extension.toml`:

```toml
[security]
capabilities = ["providers.register"]

[[providers.ide]]
name = "my-driver"
command = "/path/to/binary"

[providers.ide.metadata]
display_name = "My Driver"
fixed_args = "--acp"
probe_args = "--version"
default_model = "my-model"
supports_add_dirs = "true"
uses_bootstrap_model = "false"
env.MY_VAR = "value"
```

The chain is:

```
extension.toml
  -> ExtractDeclaredProviders()           (assets.go:42-59)
    -> agentOverlayEntries()              (extensions_bootstrap.go:63-77)
      -> agent.ActivateOverlay()          (registry_overlay.go:33-49)
        -> specFromDeclaredIDEProvider()  (registry_overlay.go:113-161)
          -> merged into catalogSnapshot
```

### Gaps in IDE Provider Overlay

**Gap 1: `BootstrapArgs` callback not declarable**

This is a Go function, not declarable via manifest metadata. Built-in drivers like Droid use it to dynamically construct `--model` and `--reasoning-effort` args. Overlay-declared drivers get `BootstrapArgs = nil`.

Current built-in usage example (Droid, registry_specs.go):

```go
BootstrapArgs: func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string {
    args := []string{"--model", modelName}
    if reasoningEffort != "" {
        args = append(args, "--reasoning-effort", reasoningEffort)
    }
    // ... addDirs handling
    return args
}
```

**Gap 2: `Fallbacks []Launcher` not declarable**

Alternative launcher commands (e.g., `npx` fallback for a Node-based driver). Not parseable from metadata. Overlay-declared drivers get empty fallbacks.

```go
type Launcher struct {
    Command   string
    FixedArgs []string
    ProbeArgs []string
}
```

**Gap 3: TUI form is hardcoded**

`internal/cli/form.go` lines 375-393:

```go
func (fb *formBuilder) addIDEField(target *string) {
    fb.addField("ide", func() huh.Field {
        return huh.NewSelect[string]().
            Key("ide").
            Title("IDE Tool").
            Description("Choose which ACP runtime to use...").
            Options(
                huh.NewOption("Codex", string(core.IDECodex)),
                huh.NewOption("Claude", string(core.IDEClaude)),
                huh.NewOption("Cursor", string(core.IDECursor)),
                huh.NewOption("Droid", string(core.IDEDroid)),
                huh.NewOption("OpenCode", string(core.IDEOpenCode)),
                huh.NewOption("Pi", string(core.IDEPi)),
                huh.NewOption("Gemini", string(core.IDEGemini)),
                huh.NewOption("Copilot CLI", string(core.IDECopilot)),
            ).
            Value(target)
    })
}
```

This does NOT call `DriverCatalog()` or `currentCatalogSnapshot()`. Overlay-declared drivers never appear in the interactive form.

**Gap 4: Bootstrap timing mismatch**

In `internal/cli/run.go` (lines 28-79), the form is shown **before** overlay activation:

```
prepareAndRun() {
    applyWorkspaceDefaults()                    // line 36
    setupFn -> maybeCollectInteractiveParams()  // line 43 <- FORM SHOWN HERE
    buildConfig()                               // line 59
    bootstrapDeclarativeAssets()                // line 66 <- OVERLAY ACTIVATED HERE (too late)
    runPrepared()                               // line 75
}
```

The overlay is activated AFTER the interactive form has already been displayed and collected user input.

---

## Provider Type 2: Review Providers

### Current Implementation

One built-in provider: CodeRabbit (`internal/core/provider/coderabbit/`).

The `Provider` interface (`internal/core/provider/provider.go`):

```go
type Provider interface {
    Name() string
    FetchReviews(ctx context.Context, req FetchRequest) ([]ReviewItem, error)
    ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error
}

type FetchRequest struct {
    PR              string
    IncludeNitpicks bool
}

type ReviewItem struct {
    Title       string
    File        string
    Line        int
    Severity    string
    Author      string
    Body        string
    ProviderRef string
    ReviewHash              string
    SourceReviewID          string
    SourceReviewSubmittedAt string
}

type ResolvedIssue struct {
    FilePath    string
    ProviderRef string
}
```

Providers are stored in a `Registry` (`provider/registry.go`) and resolved via `provider.ResolveRegistry()`.

### Extension Overlay Mechanism (Aliasing Only)

An extension can declare:

```toml
[[providers.review]]
name = "my-review-provider"
command = "coderabbit"
```

The chain:

```
extension.toml
  -> ExtractDeclaredProviders()
    -> providerOverlayEntries()              (extensions_bootstrap.go:79-93)
      -> provider.ActivateOverlay()          (overlay.go:71-90)
        -> buildDeclaredReviewOverlay()      (overlay.go:103-113)
```

### Critical Gap: Aliasing Only, No New Implementations

The `command` field is treated as a **target provider name for aliasing**, not as an executable command:

```go
func buildDeclaredReviewOverlay(base RegistryReader, entries []OverlayEntry) RegistryReader {
    overlay := NewOverlayRegistry(base)
    for _, entry := range entries {
        overlay.Register(&aliasedProvider{
            name:       strings.TrimSpace(entry.Name),
            targetName: strings.TrimSpace(entry.Command), // <- this is a NAME, not an executable
            registry:   overlay,
        })
    }
    return overlay
}
```

The `aliasedProvider` simply delegates `FetchReviews()` and `ResolveIssues()` to the named target provider:

```go
type aliasedProvider struct {
    name       string
    targetName string
    registry   RegistryReader
}

func (p *aliasedProvider) FetchReviews(ctx context.Context, req FetchRequest) ([]ReviewItem, error) {
    target, err := p.resolveTarget(nil)
    return target.FetchReviews(ctx, req)
}

func (p *aliasedProvider) ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error {
    target, err := p.resolveTarget(nil)
    return target.ResolveIssues(ctx, pr, issues)
}
```

**Extensions CANNOT add genuinely new review provider implementations** -- they can only alias/rename existing ones. There is no JSON-RPC method for extensions to implement `FetchReviews` or `ResolveIssues` operations.

---

## Provider Type 3: Model Providers

### Current State: Completely Unimplemented

- Manifest supports `[[providers.model]]` declarations
- Discovery extracts them into `DiscoveryResult.Providers.Model`
- `extensions_bootstrap.go` **ignores** `discovery.Providers.Model` entirely -- no activation code
- No overlay mechanism for model providers
- No runtime resolution path
- Model is an opaque string passed through the stack with no provider abstraction

In the current architecture, model is just a `string` field on `RuntimeConfig` that gets passed to the ACP agent via `SetSessionModel()`. There is no model provider interface, no validation, no routing.

Model constants (`internal/core/model/constants.go`):

```go
const (
    DefaultCodexModel    = "gpt-5.4"
    DefaultClaudeModel   = "opus"
    DefaultCursorModel   = "composer-1"
    DefaultOpenCodeModel = "anthropic/claude-opus-4-6"
    DefaultPiModel       = "anthropic/claude-opus-4-6"
    DefaultGeminiModel   = "gemini-2.5-pro"
    DefaultCopilotModel  = "claude-sonnet-4.6"
)
```

No model provider abstraction, validation, or routing exists.

---

## Capability and Host API Context

### `providers.register` Capability

The capability exists in `manifest.go` line 50:

```go
CapabilityProvidersRegister Capability = "providers.register"
```

It is only enforced at **manifest validation time** (install-time check in `manifest_validate.go` lines 180-213). There is no runtime enforcement -- no host API method requires this capability.

### Host API Router Structure

From `internal/core/extension/host_api.go` (lines 36-142):

- Routes follow `host.<namespace>.<verb>` pattern
- 6 namespaces registered: events, tasks, runs, artifacts, prompts, memory
- Method parsing: `splitHostMethod` expects 3 parts: `["host", namespace, verb]`
- Capability checked via `extension.Capabilities.CheckHostMethod(method)` before dispatch
- Adding a new namespace requires: handler function in host_reads.go or host_writes.go, capability mapping in capability.go, registration in host_helpers.go

### Existing Capability Mapping

From `internal/core/extension/capability.go` (lines 19-31):

```go
var hostMethodCapabilities = map[string]Capability{
    "host.events.subscribe": CapabilityEventsRead,
    "host.events.publish":   CapabilityEventsPublish,
    "host.tasks.list":       CapabilityTasksRead,
    "host.tasks.get":        CapabilityTasksRead,
    "host.tasks.create":     CapabilityTasksCreate,
    "host.runs.start":       CapabilityRunsStart,
    "host.artifacts.read":   CapabilityArtifactsRead,
    "host.artifacts.write":  CapabilityArtifactsWrite,
    "host.prompts.render":   "",  // No capability required
    "host.memory.read":      CapabilityMemoryRead,
    "host.memory.write":     CapabilityMemoryWrite,
}
```

No `host.providers.*` entries exist.

---

## Extension Hook System for Reviews

Review hooks exist and are dispatched during the review workflow:

```go
HookReviewPreFetch   HookName = "review.pre_fetch"    // mutable: can mutate FetchConfig
HookReviewPostFetch  HookName = "review.post_fetch"    // mutable: can mutate Issues list
HookReviewPreBatch   HookName = "review.pre_batch"     // mutable: can mutate grouped Issues
HookReviewPostFix    HookName = "review.post_fix"      // observer: outcome feedback
HookReviewPreResolve HookName = "review.pre_resolve"   // mutable: can mutate resolve decision
```

These hooks allow extensions to **mutate data flowing through the review pipeline** but do NOT replace the provider itself. A provider must exist in the registry for reviews to be fetched in the first place.

---

## SDK Surface

### Go SDK (`sdk/extension/`)

Handler interfaces:

```go
type HostAPI struct {
    Events    *EventsClient
    Tasks     *TasksClient
    Runs      *RunsClient
    Artifacts *ArtifactsClient
    Prompts   *PromptsClient
    Memory    *MemoryClient
}
```

No `Providers` client exists.

Hook registration via fluent API: `e.OnPlanPreDiscover(handler)`, `e.OnPromptPostBuild(handler)`, etc.

### TypeScript SDK (`sdk/extension-sdk-ts/`)

Same 6 namespaces. Constants define all 19 capabilities and 28 hooks. Templates include a `review-provider` template, but it only declares the manifest -- the actual review provider subprocess protocol for `FetchReviews`/`ResolveIssues` is not implemented.

---

## Reusable Agents and RuntimeDefaults

Reusable agents (`internal/core/agents/`) can override IDE and model selection:

```go
type RuntimeDefaults struct {
    IDE             string
    Model           string
    ReasoningEffort string
    AccessMode      string
}
```

Precedence: explicit CLI flags > reusable agent defaults > workspace config > spec defaults. Applied via `applyRuntimePrecedence()` in `agents/execution.go`.

---

## Constraints

- Compozy is a single-binary, local-first CLI tool
- Extensions run as subprocesses with JSON-RPC 2.0 over stdin/stdout
- Protocol version is 1.0, shipped in lockstep with SDKs
- No restart semantics -- every run gets fresh subprocesses
- Synchronous hook timeout: 5s default
- Extension states: Loaded -> Initializing -> Ready -> Draining -> Stopped
- The TUI uses Bubble Tea (`charm.land/huh/v2`) for interactive forms
- All agent communication goes through the ACP (Agent Client Protocol)
- The `internal/core/extension/builtin/` directory exists and is ready for bundled extensions but is intentionally empty in v1

---

## Requirements

1. Extensions must be able to register **genuinely new** ACP drivers that appear in the TUI form, work with `--ide` flag, and are fully functional
2. Extensions must be able to register **genuinely new** review providers that implement `FetchReviews` and `ResolveIssues` via the extension subprocess protocol
3. The TUI interactive form must dynamically reflect all available drivers (built-in + extension-declared)
4. Bootstrap timing must be corrected so extension discovery happens before interactive form display
5. The `BootstrapArgs` and `Fallbacks` gaps for IDE provider overlay must be addressed
6. The `providers.register` capability must have runtime enforcement, not just install-time validation
7. Both Go and TypeScript SDKs must be updated with the new provider registration surface
8. Model provider overlay activation should be wired up (even if minimal in v1)
9. 100% backward compatibility -- all existing CLI commands, flags, and workflows must continue working identically
