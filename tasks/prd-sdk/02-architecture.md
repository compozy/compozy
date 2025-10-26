# Architecture: Compozy GO SDK

**Date:** 2025-01-25
**Version:** 2.0.0
**Estimated Reading Time:** 20 minutes

---

## Overview

This document details the complete architecture for the Compozy GO SDK using a **Go workspace approach** with a dedicated `sdk/` module for high-level SDK components.

**Key Principles:**
- ✅ Zero changes to existing codebase
- ✅ Go workspace for unified development
- ✅ Independent sdk module versioning
- ✅ Clean separation between internal engine and public SDK
- ✅ Context-first architecture (logger and config from context)
- ✅ Direct integration with engine (no YAML intermediate)

---

## Go Workspace Structure

### Complete Repository Layout

```
compozy/
├── go.work                          # Workspace definition
├── go.work.sum                      # Workspace dependency checksums
│
├── go.mod                           # Existing module: github.com/compozy/compozy
├── go.sum
├── main.go                          # Server entrypoint (unchanged)
├── cmd/                             # CLI commands (unchanged)
│
├── engine/                          # UNCHANGED: All existing code (100+ packages)
│   ├── core/                       # Core domain types
│   ├── agent/                      # Agent execution
│   ├── task/                       # Task execution system (9 types)
│   ├── workflow/                   # Workflow orchestration
│   ├── project/                    # Project configuration
│   ├── knowledge/                  # Knowledge/RAG system
│   ├── memory/                     # Memory/conversation state
│   ├── mcp/                        # MCP integration
│   ├── llm/                        # LLM provider integration
│   ├── tool/                       # Tool execution
│   ├── runtime/                    # Runtime (Bun, Node, Deno)
│   ├── schema/                     # Schema validation
│   ├── infra/                      # Infrastructure (DB, HTTP, etc.)
│   └── ... (100+ packages)
│
├── sdk/                              # NEW: High-level SDK module
│   ├── go.mod                      # Module: github.com/compozy/compozy/sdk
│   ├── go.sum
│   ├── doc.go                      # Package documentation
│   │
│   ├── project/                    # Project configuration builder
│   ├── model/                      # Model configuration builder
│   ├── workflow/                   # Workflow builder
│   ├── agent/                      # Agent builder + ActionBuilder
│   ├── task/                       # Task builders (9 types)
│   │   ├── basic.go               # Basic task
│   │   ├── parallel.go            # Parallel task
│   │   ├── collection.go          # Collection task
│   │   ├── router.go              # Router task (switch/conditional)
│   │   ├── wait.go                # Wait task
│   │   ├── aggregate.go           # Aggregate task
│   │   ├── composite.go           # Composite task
│   │   ├── signal.go              # Signal task (send/wait unified)
│   │   └── memory.go              # Memory task
│   │
│   ├── knowledge/                  # Knowledge system builders
│   │   ├── embedder.go            # Embedder configuration
│   │   ├── vectordb.go            # Vector DB configuration
│   │   ├── source.go              # Source builder (file, dir, URL, API)
│   │   ├── base.go                # Knowledge base builder
│   │   └── binding.go             # Knowledge binding (agent attachment)
│   │
│   ├── memory/                     # Memory system builders
│   │   ├── config.go              # Memory resource configuration (full features)
│   │   └── reference.go           # Memory reference (agent attachment)
│   │
│   ├── mcp/                        # MCP integration builder (full config)
│   ├── runtime/                    # Runtime configuration builder
│   │   ├── builder.go
│   │   └── native_tools.go        # Native tools (call_agents, call_workflows)
│   │
│   ├── tool/                       # Tool builder
│   ├── schema/                     # Schema builder with validation
│   ├── schedule/                   # Schedule builder
│   ├── monitoring/                 # Monitoring builder (full Prometheus config)
│   │
│   ├── compozy/                    # Main SDK package for embedding Compozy
│   │   ├── compozy.go             # Main Compozy struct and lifecycle
│   │   ├── builder.go             # Builder for configuration
│   │   ├── integration.go         # SDK → Engine integration layer
│   │   ├── execution.go           # Direct workflow execution
│   │   └── types.go
│   │
│   ├── internal/                   # Internal utilities (not public API)
│   │   ├── validate/              # Validation helpers
│   │   └── errors/                # Error handling utilities (BuildError)
│   │
│   └── examples/                   # SDK usage examples
│
└── examples/                        # UNCHANGED: Existing YAML examples
```

---

## Module Dependencies & Integration

### sdk/go.mod Definition

```go
module github.com/compozy/compozy/sdk

go 1.25.2

require (
    // Direct dependency on main module
    github.com/compozy/compozy v0.0.0
    
    // Minimal external dependencies (avoid duplication)
    github.com/stretchr/testify v1.9.0  // Testing only
)

// When published, version coupling:
// sdk v1.0.0 requires github.com/compozy/compozy v1.0.0+
```

### Workspace Configuration (go.work)

```go
go 1.25.2

use (
    .       // Existing module (github.com/compozy/compozy)
    ./sdk    // New SDK module (github.com/compozy/compozy/sdk)
)

// Workspace automatically resolves local modules
// No replace directives needed during development
```

### Import Strategy

**SDK imports from main module:**
```go
// sdk/workflow/builder.go
package workflow

import (
    "context"
    
    // Import engine types
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/task"
    
    // Import config and logger helpers
    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
)
```

**Key Rules:**
- ✅ sdk can import `engine/*` packages (types, configs)
- ✅ sdk can import `pkg/*` packages (config, logger, utilities)
- ❌ engine packages NEVER import sdk (one-way dependency)
- ✅ Within sdk: packages can import each other
- ❌ External code CANNOT import `sdk/internal/*`

---

## Context-First Architecture

### Mandatory Pattern

**All builder `Build()` methods MUST accept `context.Context`:**

```go
// CORRECT: Context-first pattern
func (b *Builder) Build(ctx context.Context) (*Config, error) {
    // Access logger from context (NEVER pass as parameter)
    log := logger.FromContext(ctx)
    log.Info("building workflow", "id", b.config.ID)
    
    // Access config from context (NEVER use global singleton)
    appConfig := config.FromContext(ctx)
    
    // Validate with context
    if err := b.config.Validate(ctx); err != nil {
        return nil, err
    }
    
    return b.config, nil
}

// WRONG: Missing context
func (b *Builder) Build() (*Config, error) {  // ❌ NO CONTEXT
    return b.config, nil
}
```

### Context Propagation Rules

1. **Logger Access:** `logger.FromContext(ctx)` - NEVER pass logger as parameter
2. **Config Access:** `config.FromContext(ctx)` - NEVER use global config singleton
3. **Validation:** All `Validate()` methods accept `context.Context`
4. **I/O Operations:** All methods performing I/O must accept context
5. **Tests:** Use `t.Context()` instead of `context.Background()`

### Example Usage

```go
package main

import (
    "context"
    
    "github.com/compozy/compozy/sdk/workflow"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/pkg/config"
)

func main() {
    // Create context with logger and config attached
    ctx := context.Background()
    ctx = logger.WithLogger(ctx, logger.New())
    ctx = config.WithConfig(ctx, config.Load())
    
    // Build workflow (context required)
    wf, err := workflow.New("my-workflow").
        WithDescription("My workflow").
        Build(ctx)  // ✅ Context passed
    
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## Integration Layer: SDK → Engine

### Architecture Decision

**Direct Integration (Option B):** SDK builders create engine config structs directly, bypassing YAML.

```
┌─────────────────┐
│   User Code     │
│   (SDK)      │
└────────┬────────┘
         │ Build(ctx)
         ▼
┌─────────────────┐
│ sdk Builders     │
│ (Fluent API)    │
└────────┬────────┘
         │ Returns engine types
         ▼
┌─────────────────────────┐
│ Engine Config Structs   │
│ (*core.Workflow, etc.)  │
└────────┬────────────────┘
         │ Direct validation
         ▼
┌─────────────────────┐
│ sdk/compozy Package  │
│ Integration Layer   │
└────────┬────────────┘
         │ Register resources
         ▼
┌─────────────────────┐
│ engine/infra/server │
│ (Dependencies, etc.)│
└─────────────────────┘
```

### Integration Layer Implementation

**Location:** `sdk/compozy/integration.go`

```go
package compozy

import (
    "context"
    "fmt"
    
    "github.com/compozy/compozy/engine/infra/server"
    "github.com/compozy/compozy/engine/infra/store"
    "github.com/compozy/compozy/engine/project"
    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
)

// loadProjectIntoEngine registers SDK-built project config into the engine
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error {
    log := logger.FromContext(ctx)
    log.Info("loading SDK project into engine", "project", proj.Name)
    
    // 1. Validate project configuration
    if err := proj.Validate(ctx); err != nil {
        return fmt.Errorf("project validation failed: %w", err)
    }
    
    // 2. Register project in resource store
    resourceStore := c.server.ResourceStore()
    if err := resourceStore.RegisterProject(ctx, proj); err != nil {
        return fmt.Errorf("failed to register project: %w", err)
    }
    
    // 3. Register all workflows
    for _, wf := range proj.Workflows {
        if err := resourceStore.RegisterWorkflow(ctx, wf); err != nil {
            return fmt.Errorf("failed to register workflow %s: %w", wf.ID, err)
        }
    }
    
    // 4. Register all agents
    for _, agent := range proj.Agents {
        if err := resourceStore.RegisterAgent(ctx, agent); err != nil {
            return fmt.Errorf("failed to register agent %s: %w", agent.ID, err)
        }
    }
    
    // 5. Register all tools
    for _, tool := range proj.Tools {
        if err := resourceStore.RegisterTool(ctx, tool); err != nil {
            return fmt.Errorf("failed to register tool %s: %w", tool.ID, err)
        }
    }
    
    // 6. Register knowledge bases
    for _, kb := range proj.KnowledgeBases {
        if err := resourceStore.RegisterKnowledgeBase(ctx, kb); err != nil {
            return fmt.Errorf("failed to register knowledge base %s: %w", kb.ID, err)
        }
    }
    
    // 7. Register memory configs
    for _, mem := range proj.Memories {
        if err := resourceStore.RegisterMemory(ctx, mem); err != nil {
            return fmt.Errorf("failed to register memory %s: %w", mem.ID, err)
        }
    }
    
    // 8. Register MCP servers
    for _, mcp := range proj.MCPs {
        if err := resourceStore.RegisterMCP(ctx, mcp); err != nil {
            return fmt.Errorf("failed to register MCP %s: %w", mcp.ID, err)
        }
    }
    
    // 9. Register schemas
    for _, schema := range proj.Schemas {
        if err := resourceStore.RegisterSchema(ctx, schema); err != nil {
            return fmt.Errorf("failed to register schema %s: %w", GetID(schema), err)
        }
    }
    
    log.Info("SDK project loaded successfully", "workflows", len(proj.Workflows))
    return nil
}
```

### Resource Store Integration

**SDK-defined resources** are registered in the engine's resource store with unique IDs:
- ID assignment handled by builder or auto-generated
- `$ref` resolution works for SDK-defined resources
- Hybrid projects: SDK + YAML resources coexist
- AutoLoad disabled for SDK projects (explicit registration only)

---

## Error Handling Strategy

### Approach: Accumulate Errors, Report at Build()

**Pattern:**
```go
type Builder struct {
    config *Config
    errors []error  // Accumulate errors during building
}

// With* methods store errors instead of returning them
func (b *Builder) WithField(value string) *Builder {
    if value == "" {
        b.errors = append(b.errors, fmt.Errorf("field cannot be empty"))
    }
    b.config.Field = value
    return b  // Always return builder for fluent API
}

// Build() returns all accumulated errors
func (b *Builder) Build(ctx context.Context) (*Config, error) {
    // Check accumulated errors first
    if len(b.errors) > 0 {
        return nil, &BuildError{Errors: b.errors}
    }
    
    // Validate configuration
    if err := b.validate(ctx); err != nil {
        return nil, err
    }
    
    return b.config, nil
}
```

### BuildError Type

**Location:** `sdk/internal/errors/build_error.go`

```go
package errors

import (
    "fmt"
    "strings"
)

// BuildError aggregates multiple build-time errors
type BuildError struct {
    Errors []error
}

func (e *BuildError) Error() string {
    if len(e.Errors) == 0 {
        return "build failed"
    }
    
    if len(e.Errors) == 1 {
        return fmt.Sprintf("build failed: %v", e.Errors[0])
    }
    
    var msgs []string
    for i, err := range e.Errors {
        msgs = append(msgs, fmt.Sprintf("  %d. %v", i+1, err))
    }
    
    return fmt.Sprintf("build failed with %d errors:\n%s", 
        len(e.Errors), strings.Join(msgs, "\n"))
}

// Unwrap returns the first error for errors.Is/As compatibility
func (e *BuildError) Unwrap() error {
    if len(e.Errors) > 0 {
        return e.Errors[0]
    }
    return nil
}
```

### Error Handling Example

```go
// Usage with fluent API
wf, err := workflow.New("").  // ❌ Empty ID (error stored)
    WithDescription("").       // ❌ Empty description (error stored)
    AddAgent(nil).             // ❌ Nil agent (error stored)
    Build(ctx)

if err != nil {
    // BuildError with all 3 errors reported
    fmt.Println(err)
    // Output:
    // build failed with 3 errors:
    //   1. workflow ID is required
    //   2. workflow description is required
    //   3. agent cannot be nil
}
```

---

## Task Type Definitions (9 Types)

### Engine Task Types (Authoritative)

```go
// From engine/task
const (
    TaskTypeBasic      task.Type = "basic"       // Single agent/tool execution
    TaskTypeParallel   task.Type = "parallel"    // Parallel task execution
    TaskTypeCollection task.Type = "collection"  // Iterate over collection
    TaskTypeRouter     task.Type = "router"      // Conditional routing (switch)
    TaskTypeWait       task.Type = "wait"        // Wait for duration/condition
    TaskTypeAggregate  task.Type = "aggregate"   // Aggregate results
    TaskTypeComposite  task.Type = "composite"   // Nested workflow
    TaskTypeSignal     task.Type = "signal"      // Signal send/wait
    TaskTypeMemory     task.Type = "memory"      // Memory operations
)
```

### SDK Task Builders

**All 9 types implemented:**

```go
// sdk/task/ package structure
sdk/task/
├── basic.go        // BasicBuilder
├── parallel.go     // ParallelBuilder
├── collection.go   // CollectionBuilder
├── router.go       // RouterBuilder (handles switch/conditional logic)
├── wait.go         // WaitBuilder
├── aggregate.go    // AggregateBuilder
├── composite.go    // CompositeBuilder
├── signal.go       // SignalBuilder (unified send/wait)
└── memory.go       // MemoryTaskBuilder
```

### Task Builder Example

```go
// sdk/task/router.go
package task

import (
    "context"
    
    "github.com/compozy/compozy/engine/task"
)

// RouterBuilder creates conditional routing tasks (switch logic)
type RouterBuilder struct {
    config *task.Config
    errors []error
}

// NewRouter creates a router task for conditional execution
func NewRouter(id string) *RouterBuilder {
    return &RouterBuilder{
        config: &task.Config{
            ID:   id,
            Type: task.TaskTypeRouter,
        },
    }
}

// WithCondition sets the routing condition
func (b *RouterBuilder) WithCondition(condition string) *RouterBuilder {
    b.config.Condition = condition
    return b
}

// AddRoute adds a conditional route
func (b *RouterBuilder) AddRoute(condition string, taskID string) *RouterBuilder {
    if b.config.Routes == nil {
        b.config.Routes = make(map[string]string)
    }
    b.config.Routes[condition] = taskID
    return b
}

// Build validates and returns the router task
func (b *RouterBuilder) Build(ctx context.Context) (*task.Config, error) {
    if len(b.errors) > 0 {
        return nil, &BuildError{Errors: b.errors}
    }
    
    if err := b.config.Validate(ctx); err != nil {
        return nil, err
    }
    
    return b.config, nil
}
```

---

## Native Tools Integration

### Runtime Native Tools

**Location:** `sdk/runtime/native_tools.go`

```go
package runtime

import (
    "github.com/compozy/compozy/engine/runtime"
)

// NativeToolsBuilder configures built-in native tools
type NativeToolsBuilder struct {
    config *runtime.NativeToolsConfig
}

// NewNativeTools creates a native tools builder
func NewNativeTools() *NativeToolsBuilder {
    return &NativeToolsBuilder{
        config: &runtime.NativeToolsConfig{},
    }
}

// WithCallAgents enables the call_agents native tool
// Allows agents to invoke other agents dynamically
func (b *NativeToolsBuilder) WithCallAgents() *NativeToolsBuilder {
    b.config.CallAgents = true
    return b
}

// WithCallWorkflows enables the call_workflows native tool
// Allows workflows to invoke other workflows dynamically
func (b *NativeToolsBuilder) WithCallWorkflows() *NativeToolsBuilder {
    b.config.CallWorkflows = true
    return b
}

// Build returns the native tools configuration
func (b *NativeToolsBuilder) Build(ctx context.Context) *runtime.NativeToolsConfig {
    // No I/O here, but keep context for consistency with SDK patterns
    return b.config
}
```

### Runtime Builder Integration

```go
// sdk/runtime/builder.go
func (b *Builder) WithNativeTools(tools *runtime.NativeToolsConfig) *Builder {
    b.config.NativeTools = tools
    return b
}

// Usage example
runtime := runtime.New(runtime.RuntimeTypeBun).
    WithNativeTools(
        nativetools.NewNativeTools().
            WithCallAgents().
            WithCallWorkflows().
            Build(ctx),
    ).
    Build(ctx)
```

---

## Memory System (Full Features)

### Memory Builder (Complete)

```go
// sdk/memory/config.go
package memory

import (
    "context"
    "time"
    
    "github.com/compozy/compozy/engine/memory"
)

type ConfigBuilder struct {
    config *memory.Config
    errors []error
}

func New(id string) *ConfigBuilder {
    return &ConfigBuilder{
        config: &memory.Config{ID: id},
    }
}

// Core configuration
func (b *ConfigBuilder) WithProvider(provider string) *ConfigBuilder
func (b *ConfigBuilder) WithModel(model string) *ConfigBuilder
func (b *ConfigBuilder) WithMaxTokens(max int) *ConfigBuilder

// Flush strategies
func (b *ConfigBuilder) WithFlushStrategy(strategy memory.FlushStrategy) *ConfigBuilder
func (b *ConfigBuilder) WithFIFOFlush(maxMessages int) *ConfigBuilder
func (b *ConfigBuilder) WithSummarizationFlush(provider, model string) *ConfigBuilder

// Privacy and security
func (b *ConfigBuilder) WithPrivacy(privacy memory.PrivacyScope) *ConfigBuilder
func (b *ConfigBuilder) WithExpiration(duration time.Duration) *ConfigBuilder

// Persistence
func (b *ConfigBuilder) WithPersistence(backend memory.PersistenceBackend) *ConfigBuilder

// Token counting
func (b *ConfigBuilder) WithTokenCounter(provider, model string) *ConfigBuilder

// Build with context
func (b *ConfigBuilder) Build(ctx context.Context) (*memory.Config, error)
```

### Example: Advanced Memory Configuration

```go
memory := memory.New("customer-support").
    WithProvider("openai").
    WithModel("gpt-4o-mini").
    WithMaxTokens(2000).
    WithSummarizationFlush("openai", "gpt-4").  // Use GPT-4 for summaries
    WithPrivacy(memory.PrivacyUserScope).        // Isolate by user
    WithExpiration(24 * time.Hour).              // Auto-expire after 24h
    WithPersistence(memory.PersistenceRedis).    // Store in Redis
    Build(ctx)
```

---

## MCP Integration (Full Features)

### MCP Builder (Complete)

```go
// sdk/mcp/builder.go
package mcp

import (
    "context"
    "time"
    
    "github.com/compozy/compozy/engine/mcp"
    mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

type Builder struct {
    config *mcp.Config
    errors []error
}

func New(id string) *Builder {
    return &Builder{
        config: &mcp.Config{ID: id},
    }
}

// Command-based MCP (stdio transport)
func (b *Builder) WithCommand(command string, args ...string) *Builder

// URL-based MCP (SSE/HTTP transport)
func (b *Builder) WithURL(url string) *Builder

// Transport configuration
func (b *Builder) WithTransport(transport mcpproxy.TransportType) *Builder

// HTTP headers (for URL-based MCPs)
func (b *Builder) WithHeaders(headers map[string]string) *Builder
func (b *Builder) WithHeader(key, value string) *Builder

// Protocol version
func (b *Builder) WithProto(version string) *Builder

// Process configuration (for command-based MCPs)
func (b *Builder) WithEnv(env map[string]string) *Builder
func (b *Builder) WithEnvVar(key, value string) *Builder
func (b *Builder) WithStartTimeout(timeout time.Duration) *Builder

// Session management
func (b *Builder) WithMaxSessions(max int) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*mcp.Config, error)
```

### Example: MCP Configurations

```go
// Stdio MCP with environment
mcpLocal := mcp.New("filesystem").
    WithCommand("mcp-server-filesystem").
    WithEnvVar("ROOT_DIR", "/data").
    WithStartTimeout(10 * time.Second).
    Build(ctx)

// Remote MCP with SSE transport
mcpRemote := mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithTransport(mcpproxy.TransportSSE).
    WithHeader("Authorization", "Bearer {{.env.GITHUB_TOKEN}}").
    WithProto("2025-03-26").
    WithMaxSessions(10).
    Build(ctx)
```

---

## Builder Immutability & Lifecycle

### Builder Lifecycle

**Builders are REUSABLE but produce independent configs:**

```go
// Builder is reusable
builder := workflow.New("base-workflow").
    WithDescription("Base workflow")

// Each Build() produces an independent config
wf1, _ := builder.AddTask(task1).Build(ctx)
wf2, _ := builder.AddTask(task2).Build(ctx)

// wf1 has only task1
// wf2 has both task1 and task2 (cumulative)
```

**Internal deep cloning (copy-on-write):**
```go
func (b *Builder) Build(ctx context.Context) (*Config, error) {
    // Deep clone config to prevent mutation
    cloned, err := b.config.Clone()
    if err != nil {
        return nil, err
    }
    
    if err := cloned.Validate(ctx); err != nil {
        return nil, err
    }
    
    return cloned, nil
}
```

---

## Performance & Concurrency

### Performance Targets

- **Builder operations:** <1ms per method call
- **Build() validation:** <10ms for typical workflow
- **Memory overhead:** <100KB per builder instance

### Concurrency Safety

**Builders are NOT thread-safe:**
```go
// ❌ UNSAFE: Multiple goroutines modifying same builder
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        builder.AddTask(task)  // ❌ RACE CONDITION
    }()
}

// ✅ SAFE: Each goroutine gets own builder
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        b := workflow.New("wf").AddTask(task).Build(ctx)  // ✅ OK
    }()
}
```

---

## Version Compatibility Matrix

### SDK ↔ Engine Versioning

| SDK Version | Engine Version | Compatibility | Notes |
|-------------|---------------|---------------|-------|
| sdk v0.1.0   | v1.0.0+       | ✅ Compatible | Initial SDK release |
| sdk v0.2.0   | v1.1.0+       | ✅ Compatible | Added signal tasks |
| sdk v1.0.0   | sdk.0.0+       | ✅ Compatible | Stable SDK release |

### Breaking Change Policy

- **Minor versions (sdk.X.0):** New features, backward compatible
- **Patch versions (sdk.0.X):** Bug fixes only
- **Major versions (v3.0.0):** Breaking changes allowed

---

## Hybrid SDK + YAML Projects

### Coexistence Strategy

SDK-defined and YAML-defined resources can coexist:

```go
// SDK defines workflows programmatically
proj := project.New("my-project").
    AddWorkflow(sdkWorkflow).
    WithAutoLoad(true, []string{"tools/*.yaml"}, nil).  // Also load YAML tools
    Build(ctx)

// Result: SDK workflows + YAML tools both registered
```

**Resource Resolution:**
1. SDK-defined resources registered first
2. AutoLoad discovers YAML resources
3. `$ref` resolves to SDK or YAML resources by ID
4. No conflicts: SDK IDs must be unique from YAML IDs

---

## Summary

### Key Architecture Decisions

1. ✅ **Go Workspace** - Unified development, zero disruption
2. ✅ **Direct Integration** - SDK → Engine (no YAML intermediate)
3. ✅ **Context-First** - Logger and config from context
4. ✅ **9 Task Types** - Complete engine task type coverage
5. ✅ **Full Feature Parity** - Memory, MCP, native tools, etc.
6. ✅ **Error Accumulation** - BuildError aggregates multiple errors
7. ✅ **Builder Reusability** - Deep cloning for independent configs
8. ✅ **Hybrid Projects** - SDK + YAML coexistence

### What Changed from v1.0

- ✅ Added sdk/go.mod explicit definition
- ✅ Added integration layer documentation (SDK → Engine)
- ✅ Added context-first pattern to all builders
- ✅ Fixed task types (9 types, not 6)
- ✅ Added native tools integration
- ✅ Expanded memory system to full features
- ✅ Expanded MCP to full configuration
- ✅ Defined error handling strategy (BuildError)
- ✅ Documented builder immutability and lifecycle
- ✅ Added version compatibility matrix
- ✅ Documented hybrid SDK+YAML projects

---

**End of Architecture Document**

**Status:** ✅ Complete (All P0 issues addressed)
**Next Document:** 03-sdk-entities.md (Complete API reference with context)
