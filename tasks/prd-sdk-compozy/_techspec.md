# Technical Specification: SDK v2 Compozy Package

## Executive Summary

**Status:** Design Phase  
**Target:** sdk/compozy - Embedded Compozy engine for Go applications  
**Timeline:** 7-9 days implementation + 2-3 days testing  
**Dependencies:** sdk/client (completed), sdk resource packages (completed)

This specification defines the architecture, interfaces, and implementation requirements for `sdk/compozy`, a comprehensive SDK package that enables developers to build and run Compozy applications entirely in Go without requiring CLI or external configuration files.

---

## 1. Goals & Problem Statement

### 1.1 Primary Goal

**Enable building complete Compozy applications in Go code without touching CLI/YAML.**

```go
// Everything defined in Go - no CLI, no YAML needed
app, _ := compozy.New(ctx,
    compozy.WithWorkflow(myWorkflow),
    compozy.WithAgent(myAgent),
)
app.Start(ctx)
result, _ := app.ExecuteWorkflowSync(ctx, "my-workflow", req)
```

### 1.2 Problem Statement

**Current State:**

- Legacy `sdk/compozy` exists but has manual boilerplate and outdated patterns
- `sdk/client` only provides HTTP client (remote execution)
- No unified way to build Compozy apps programmatically in Go
- Users must use CLI + YAML for all configuration

**Target State:**

- Single SDK package for building Compozy applications in Go
- Mode-aware (standalone/distributed) with automatic infrastructure
- Resource management via sdk constructors (`workflow.New()`, `agent.New()`)
- Execution via internal client (HTTP transport)
- Optional YAML loading for hybrid approaches

### 1.3 Success Criteria

✅ **Pure Go Development:** Build entire apps without CLI/YAML  
✅ **Mode Awareness:** Seamless standalone/distributed deployment  
✅ **SDK2 Integration:** Uses sdk resource constructors  
✅ **Client Integration:** Uses sdk/client for execution transport  
✅ **Explicit API:** Clear, type-safe, no magic behavior  
✅ **Code Generation:** 80%+ boilerplate auto-generated

---

## 2. Architecture Design

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Application                          │
│  import "github.com/compozy/sdk/compozy"                   │
│  import "github.com/compozy/sdk/workflow"                  │
│  import "github.com/compozy/sdk/agent"                     │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   sdk/compozy (Main SDK)                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Resource Management (Orchestration Layer)             │ │
│  │  • Workflow/Agent/Tool registration                    │ │
│  │  • Resource validation & dependency graph              │ │
│  │  • Lifecycle management (Start/Stop/Wait)              │ │
│  │  • Mode management (standalone/distributed)            │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│                          ▼                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Execution Layer (uses sdk/client)                    │ │
│  │  • ExecuteWorkflow{Sync,Stream}()                      │ │
│  │  • ExecuteTask{Sync,Stream}()                          │ │
│  │  • ExecuteAgent{Sync,Stream}()                         │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
└──────────────────────────┼───────────────────────────────────┘
                          │
                          ▼
           ┌──────────────────────────────┐
           │      sdk/client              │
           │  (HTTP Transport Layer)       │
           └──────────────────────────────┘
                          │
                          ▼
           ┌──────────────────────────────┐
           │   HTTP API (embedded OR      │
           │   remote Compozy server)     │
           └──────────────────────────────┘
```

### 2.2 Component Responsibilities

#### 2.2.1 Engine (Orchestration)

- Collects and manages resources (workflows, agents, tools, etc.)
- Validates cross-references and dependency graphs
- Manages lifecycle (Start/Stop/Wait)
- Provisions infrastructure based on mode (standalone/distributed)
- Exposes execution methods (delegates to client)

#### 2.2.2 Client (Transport)

- HTTP communication layer
- Handles async, sync, and streaming execution
- Session management and error handling
- Already implemented in `sdk/client`

#### 2.2.3 Mode Manager

- Determines deployment mode (standalone/distributed)
- Validates mode-specific configuration requirements
- Starts/stops embedded services (Temporal, Redis) in standalone mode
- Connects to external services in distributed mode

#### 2.2.4 Resource Manager

- Registers resources from sdk constructors
- Persists to resource store (memory or Redis)
- Validates resource schemas
- Detects and prevents duplicates

#### 2.2.5 Validation Engine

- Cross-reference validation (agent references, tool references, etc.)
- Dependency graph construction and analysis
- Circular dependency detection
- Missing reference detection

---

## 3. Core Interfaces & Contracts

### 3.1 Engine Interface

```go
// Engine is the main interface for Compozy SDK
type Engine interface {
    // Lifecycle Management
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Wait()

    // Workflow Execution
    ExecuteWorkflow(ctx context.Context, workflowID string, req *ExecuteRequest) (*ExecuteResponse, error)
    ExecuteWorkflowSync(ctx context.Context, workflowID string, req *ExecuteSyncRequest) (*ExecuteSyncResponse, error)
    ExecuteWorkflowStream(ctx context.Context, workflowID string, req *ExecuteRequest, opts *client.StreamOptions) (*client.StreamSession, error)

    // Task Execution
    ExecuteTask(ctx context.Context, taskID string, req *ExecuteRequest) (*ExecuteResponse, error)
    ExecuteTaskSync(ctx context.Context, taskID string, req *ExecuteSyncRequest) (*ExecuteSyncResponse, error)
    ExecuteTaskStream(ctx context.Context, taskID string, req *ExecuteRequest, opts *client.StreamOptions) (*client.StreamSession, error)

    // Agent Execution
    ExecuteAgent(ctx context.Context, agentID string, req *ExecuteRequest) (*ExecuteResponse, error)
    ExecuteAgentSync(ctx context.Context, agentID string, req *ExecuteSyncRequest) (*ExecuteSyncResponse, error)
    ExecuteAgentStream(ctx context.Context, agentID string, req *ExecuteRequest, opts *client.StreamOptions) (*client.StreamSession, error)

    // Dynamic Resource Loading (Runtime)
    LoadProject(path string) error
    LoadWorkflow(path string) error
    LoadWorkflowsFromDir(dir string) error
    LoadAgent(path string) error
    LoadAgentsFromDir(dir string) error
    LoadTool(path string) error
    LoadToolsFromDir(dir string) error
    LoadKnowledge(path string) error
    LoadKnowledgeFromDir(dir string) error
    LoadMemory(path string) error
    LoadMemoriesFromDir(dir string) error
    LoadMCP(path string) error
    LoadMCPFromDir(dir string) error
    LoadSchema(path string) error
    LoadSchemasFromDir(dir string) error
    LoadModel(path string) error
    LoadModelsFromDir(dir string) error
    LoadSchedule(path string) error
    LoadSchedulesFromDir(dir string) error
    LoadWebhook(path string) error
    LoadWebhooksFromDir(dir string) error

    // Programmatic Resource Registration (Runtime)
    RegisterProject(cfg *project.Config) error
    RegisterWorkflow(cfg *workflow.Config) error
    RegisterAgent(cfg *agent.Config) error
    RegisterTool(cfg *tool.Config) error
    RegisterKnowledge(cfg *knowledge.Config) error
    RegisterMemory(cfg *memory.Config) error
    RegisterMCP(cfg *mcp.Config) error
    RegisterSchema(cfg *schema.Schema) error
    RegisterModel(cfg *core.ProviderConfig) error
    RegisterSchedule(cfg *schedule.Config) error
    RegisterWebhook(cfg *webhook.Config) error

    // Validation
    ValidateReferences() (*ValidationReport, error)

    // Introspection
    Server() *http.Server
    Router() *chi.Mux
    Config() *config.Config
    ResourceStore() resources.ResourceStore
    Mode() Mode
    IsStarted() bool
}
```

### 3.2 Constructor Signature

```go
// New creates a Compozy engine using functional options
func New(ctx context.Context, opts ...Option) (*Engine, error)

// Option configures the engine
type Option func(*config)
```

### 3.3 Request/Response Types (Unified)

```go
// ExecuteRequest is the unified request type for async execution
type ExecuteRequest struct {
    Input   map[string]any `json:"input,omitempty"`
    Options map[string]any `json:"options,omitempty"`
}

// ExecuteSyncRequest is for synchronous execution
type ExecuteSyncRequest struct {
    Input   map[string]any `json:"input,omitempty"`
    Options map[string]any `json:"options,omitempty"`
    Timeout *time.Duration `json:"timeout,omitempty"`
}

// ExecuteResponse is the unified response for async execution
type ExecuteResponse struct {
    ExecID  string `json:"exec_id"`
    ExecURL string `json:"exec_url"`
}

// ExecuteSyncResponse is for synchronous execution
type ExecuteSyncResponse struct {
    ExecID string         `json:"exec_id"`
    Output map[string]any `json:"output"`
}
```

### 3.4 Mode Types

```go
// Mode represents deployment mode
type Mode string

const (
    ModeStandalone  Mode = "standalone"
    ModeDistributed Mode = "distributed"
)

// StandaloneTemporalConfig configures embedded Temporal server
type StandaloneTemporalConfig struct {
    DatabaseFile  string        // SQLite file path or ":memory:"
    FrontendPort  int           // Temporal gRPC port
    BindIP        string        // IP to bind to
    Namespace     string        // Temporal namespace
    ClusterName   string        // Cluster name
    EnableUI      bool          // Enable Temporal Web UI
    UIPort        int           // UI port
    LogLevel      string        // Log level
    StartTimeout  time.Duration // Server start timeout
}

// StandaloneRedisConfig configures embedded Redis server
type StandaloneRedisConfig struct {
    Port              int           // Redis port
    Persistence       bool          // Enable persistence
    PersistenceDir    string        // Persistence directory
    SnapshotInterval  time.Duration // Snapshot interval
    MaxMemory         int64         // Max memory bytes
}

// ValidationReport contains reference validation results
type ValidationReport struct {
    Valid         bool
    Errors        []ValidationError
    Warnings      []ValidationWarning
    ResourceCount int
    CircularDeps  []CircularDependency
    MissingRefs   []MissingReference
    DependencyGraph map[string][]string
}
```

---

## 4. API Design Patterns

### 4.1 SDK2 Constructor Pattern (Following Standard)

```go
import (
    "github.com/compozy/sdk/compozy"
    "github.com/compozy/sdk/workflow"
    "github.com/compozy/sdk/agent"
    "github.com/compozy/sdk/tool"
)

func main() {
    ctx := context.Background()

    // Build resources using sdk constructors
    wf, err := workflow.New(ctx, "data-pipeline",
        workflow.WithName("Data Processing Pipeline"),
        workflow.WithDescription("Processes incoming data"),
        workflow.WithTasks([]*task.Config{...}),
    )

    ag, err := agent.New(ctx, "assistant",
        agent.WithModel("gpt-4"),
        agent.WithInstructions("You are a helpful assistant"),
        agent.WithTools([]tool.Config{...}),
    )

    tl, err := tool.New(ctx, "http-client",
        tool.WithType("http"),
        tool.WithConfig(map[string]any{...}),
    )

    // Create engine with functional options
    engine, err := compozy.New(ctx,
        compozy.WithMode(compozy.ModeStandalone),
        compozy.WithPort(8080),
        compozy.WithWorkflow(wf),
        compozy.WithAgent(ag),
        compozy.WithTool(tl),
    )

    // Start engine
    err = engine.Start(ctx)
    defer engine.Stop(ctx)

    // Execute workflow
    resp, err := engine.ExecuteWorkflowSync(ctx, "data-pipeline",
        &compozy.ExecuteSyncRequest{
            Input: map[string]any{"source": "api"},
        },
    )
}
```

### 4.2 Unified Execution API

**Same request types for all resource executions:**

```go
// Workflow execution
asyncResp, err := engine.ExecuteWorkflow(ctx, "my-workflow", &compozy.ExecuteRequest{
    Input: map[string]any{"key": "value"},
})

syncResp, err := engine.ExecuteWorkflowSync(ctx, "my-workflow", &compozy.ExecuteSyncRequest{
    Input: map[string]any{"key": "value"},
})

stream, err := engine.ExecuteWorkflowStream(ctx, "my-workflow",
    &compozy.ExecuteRequest{Input: map[string]any{"key": "value"}},
    &client.StreamOptions{Events: []string{"task.completed"}},
)

// Task execution - SAME request types!
taskResp, err := engine.ExecuteTaskSync(ctx, "my-task", &compozy.ExecuteSyncRequest{
    Input: map[string]any{"data": input},
})

// Agent execution - SAME request types!
agentResp, err := engine.ExecuteAgentSync(ctx, "assistant", &compozy.ExecuteSyncRequest{
    Input: map[string]any{"prompt": "Hello"},
})
```

### 4.3 Dynamic Resource Loading

```go
// Load resources dynamically from YAML files
engine, _ := compozy.New(ctx, compozy.WithMode(compozy.ModeStandalone))

engine.Start(ctx)

// Load project and resources at runtime
engine.LoadProject("./compozy.yaml")
engine.LoadWorkflowsFromDir("./workflows")
engine.LoadAgentsFromDir("./agents")
engine.LoadToolsFromDir("./tools")

// Validate after loading
report, err := engine.ValidateReferences()
if !report.Valid {
    log.Fatal("Validation failed")
}
```

---

## 5. File Structure

### 5.1 Package Layout

```
sdk/
├── compozy/
│   ├── generate.go                        # go:generate directives
│   │
│   ├── constructor.go                     # New() and functional options
│   ├── constructor_test.go                # Constructor tests
│   │
│   ├── engine.go                          # Engine implementation
│   ├── engine_execution.go                # Execution methods (generated)
│   ├── engine_loading.go                  # Load* methods (generated)
│   ├── engine_registration.go             # Register* methods (generated)
│   ├── engine_test.go                     # Engine tests
│   │
│   ├── types.go                           # Type definitions
│   ├── types_test.go                      # Type tests
│   │
│   ├── options.go                         # Functional option definitions
│   ├── options_generated.go               # Auto-generated options
│   │
│   ├── validation.go                      # Reference validation
│   ├── validation_test.go                 # Validation tests
│   │
│   ├── mode.go                            # Mode configuration
│   ├── mode_test.go                       # Mode tests
│   │
│   ├── standalone.go                      # Standalone mode implementation
│   ├── standalone_test.go                 # Standalone tests
│   │
│   ├── distributed.go                     # Distributed mode implementation
│   ├── distributed_test.go                # Distributed tests
│   │
│   ├── lifecycle.go                       # Start/Stop/Wait
│   ├── lifecycle_test.go                  # Lifecycle tests
│   │
│   ├── loader.go                          # YAML loading utilities
│   ├── loader_test.go                     # Loader tests
│   │
│   ├── errors.go                          # Error types
│   ├── constants.go                       # Constants
│   │
│   ├── doc.go                             # Package documentation
│   └── README.md                          # Usage guide
│
├── internal/
│   └── sdkcodegen/                        # Code generator
│       ├── cmd/
│       │   └── sdkgen/
│       │       └── main.go                # Generator CLI
│       ├── spec.go                        # Resource specifications
│       ├── options_generator.go           # Option generator
│       ├── execution_generator.go         # Execution method generator
│       ├── loading_generator.go           # Loading method generator
│       ├── registration_generator.go      # Registration method generator
│       └── README.md                      # Generator docs
```

### 5.2 Generated Files (Auto-Generated)

```
sdk/compozy/
├── options_generated.go              # With* functional options
├── engine_execution.go               # Execute* methods
├── engine_loading.go                 # Load* and Load*FromDir methods
└── engine_registration.go            # Register* methods
```

---

## 6. Code Generation Strategy

### 6.1 Resource Specifications

```go
// internal/sdkcodegen/spec.go

type ResourceSpec struct {
    Name           string   // "Workflow", "Agent", etc.
    PluralName     string   // "Workflows", "Agents", etc.
    PackagePath    string   // "github.com/compozy/compozy/engine/workflow"
    SDK2Package    string   // "github.com/compozy/sdk/workflow"
    TypeName       string   // "Config"
    BuilderField   string   // "workflows" (in engine struct)
    IsSlice        bool     // true for most, false for project
    FileExtensions []string // [".yaml", ".yml"]
}

var ResourceSpecs = []ResourceSpec{
    {
        Name:           "Workflow",
        PluralName:     "Workflows",
        PackagePath:    "github.com/compozy/compozy/engine/workflow",
        SDK2Package:    "github.com/compozy/sdk/workflow",
        TypeName:       "Config",
        BuilderField:   "workflows",
        IsSlice:        true,
        FileExtensions: []string{".yaml", ".yml"},
    },
    // ... other resources
}
```

### 6.2 Generation Commands

```bash
# From sdk/compozy directory
go generate

# Generates:
# - options_generated.go
# - engine_execution.go
# - engine_loading.go
# - engine_registration.go
```

### 6.3 What Gets Generated

**1. Functional Options (`options_generated.go`):**

```go
// WithWorkflow registers a workflow configuration
func WithWorkflow(cfg *workflow.Config) Option {
    return func(c *config) {
        c.workflows = append(c.workflows, cfg)
    }
}

// WithAgent registers an agent configuration
func WithAgent(cfg *agent.Config) Option {
    return func(c *config) {
        c.agents = append(c.agents, cfg)
    }
}
// ... all other resources
```

**2. Execution Methods (`engine_execution.go`):**

```go
func (e *Engine) ExecuteWorkflow(ctx context.Context, workflowID string, req *ExecuteRequest) (*ExecuteResponse, error) {
    clientReq := &client.WorkflowExecuteRequest{Input: req.Input, Options: req.Options}
    resp, err := e.client.ExecuteWorkflow(ctx, workflowID, clientReq)
    // ... conversion and return
}
// ... all execution methods
```

**3. Loading Methods (`engine_loading.go`):**

```go
func (e *Engine) LoadWorkflow(path string) error {
    cfg, err := loadYAML[*workflow.Config](path)
    if err != nil {
        return err
    }
    return e.RegisterWorkflow(cfg)
}

func (e *Engine) LoadWorkflowsFromDir(dir string) error {
    return loadFromDir(dir, "*.yaml", e.LoadWorkflow)
}
// ... all resources
```

**4. Registration Methods (`engine_registration.go`):**

```go
func (e *Engine) RegisterWorkflow(cfg *workflow.Config) error {
    return e.resourceStore.SaveWorkflow(e.ctx, cfg)
}
// ... all resources
```

---

## 7. Dependencies & Integration

### 7.1 Direct Dependencies

**Engine Packages:**

- `engine/infra/server` - Server initialization
- `engine/infra/cache` - Cache/Redis management
- `engine/infra/repo` - Database layer
- `engine/resources` - Resource store interface
- `engine/worker/embedded` - Embedded Temporal server
- All engine domain packages for config types

**SDK v2 Packages:**

- `sdk/client` - HTTP transport layer (CRITICAL)
- `sdk/workflow` - Workflow constructors
- `sdk/agent` - Agent constructors
- `sdk/tool` - Tool constructors
- All other SDK v2 resource packages

**Core Packages:**

- `pkg/config` - Configuration management
- `pkg/logger` - Context-based logging
- `pkg/template` - Template processing

**External:**

- `github.com/dave/jennifer/jen` - Code generation
- `github.com/go-chi/chi/v5` - HTTP router
- `gopkg.in/yaml.v3` - YAML parsing

### 7.2 Integration with sdk/client

**Critical Integration:**

```go
// Engine uses client for ALL execution
type Engine struct {
    client *client.Client  // ← HTTP transport
    // ...
}

func (e *Engine) Start(ctx context.Context) error {
    // Start embedded server (if standalone)
    if e.mode == ModeStandalone {
        e.startEmbeddedServer()
    }

    // Create client pointing to server (embedded or remote)
    baseURL := e.getBaseURL()
    e.client = client.New(ctx, baseURL)

    return nil
}

func (e *Engine) ExecuteWorkflowSync(ctx context.Context, workflowID string, req *ExecuteSyncRequest) (*ExecuteSyncResponse, error) {
    // Convert compozy request → client request
    clientReq := &client.WorkflowSyncRequest{
        Input:   req.Input,
        Timeout: req.Timeout,
    }

    // Delegate to client
    resp, err := e.client.ExecuteWorkflowSync(ctx, workflowID, clientReq)
    if err != nil {
        return nil, err
    }

    // Convert client response → compozy response
    return &ExecuteSyncResponse{
        ExecID: resp.ExecID,
        Output: resp.Output,
    }, nil
}
```

**Dependency Direction:**

```
sdk/compozy  →  depends on  →  sdk/client
     ↓                               ↓
Orchestration                   Transport
```

### 7.3 Integration with sdk Resource Packages

**Uses sdk constructors for validation:**

```go
// Users build resources with sdk constructors
wf, err := workflow.New(ctx, "my-workflow",
    workflow.WithName("My Workflow"),
    workflow.WithTasks(tasks),
)

// Compozy accepts the validated config
engine, err := compozy.New(ctx,
    compozy.WithWorkflow(wf),  // Already validated by workflow.New()
)
```

---

## 8. Use Cases

### 8.1 Pure Go Application (Primary Use Case)

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/sdk/compozy"
    "github.com/compozy/sdk/workflow"
    "github.com/compozy/sdk/agent"
    "github.com/compozy/sdk/task"
)

func main() {
    ctx := context.Background()

    // Build agent
    assistant, err := agent.New(ctx, "assistant",
        agent.WithModel("gpt-4"),
        agent.WithInstructions("You are a helpful assistant"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Build workflow
    greeting, err := workflow.New(ctx, "greeting",
        workflow.WithName("Greeting Workflow"),
        workflow.WithTasks([]*task.Config{
            {
                ID:     "greet",
                Type:   "basic",
                Agent:  &agent.Reference{ID: "assistant"},
                Action: "say_hello",
            },
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create engine
    engine, err := compozy.New(ctx,
        compozy.WithMode(compozy.ModeStandalone),
        compozy.WithPort(8080),
        compozy.WithAgent(assistant),
        compozy.WithWorkflow(greeting),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Start
    if err := engine.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer engine.Stop(ctx)

    log.Println("✅ Compozy running at http://localhost:8080")

    // Execute workflow
    result, err := engine.ExecuteWorkflowSync(ctx, "greeting",
        &compozy.ExecuteSyncRequest{
            Input: map[string]any{"name": "World"},
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Result: %v", result.Output)
}
```

### 8.2 Hybrid Approach (Go + YAML)

```go
func main() {
    ctx := context.Background()

    // Build some resources in Go
    customTool, _ := tool.New(ctx, "custom-tool",
        tool.WithType("http"),
        tool.WithConfig(map[string]any{
            "url": os.Getenv("API_URL"),
        }),
    )

    // Create engine
    engine, _ := compozy.New(ctx,
        compozy.WithMode(compozy.ModeStandalone),
        compozy.WithTool(customTool),
    )

    // Load rest from YAML
    engine.LoadProject("./compozy.yaml")
    engine.LoadWorkflowsFromDir("./workflows")
    engine.LoadAgentsFromDir("./agents")

    // Validate
    report, _ := engine.ValidateReferences()
    if !report.Valid {
        log.Fatalf("Validation failed: %v", report.Errors)
    }

    engine.Start(ctx)
    defer engine.Stop(ctx)
}
```

### 8.3 Distributed Mode (Production)

```go
func main() {
    ctx := context.Background()

    // Build resources
    workflow, _ := workflow.New(ctx, "production-workflow", ...)
    agent, _ := agent.New(ctx, "prod-agent", ...)

    // Create engine in distributed mode
    engine, err := compozy.New(ctx,
        compozy.WithMode(compozy.ModeDistributed),
        compozy.WithHost("0.0.0.0"),
        compozy.WithPort(8080),
        compozy.WithDatabaseURL(os.Getenv("DATABASE_URL")),
        compozy.WithTemporalURL(os.Getenv("TEMPORAL_URL")),
        compozy.WithTemporalNamespace("production"),
        compozy.WithRedisURL(os.Getenv("REDIS_URL")),
        compozy.WithRedisPassword(os.Getenv("REDIS_PASSWORD")),
        compozy.WithWorkflow(workflow),
        compozy.WithAgent(agent),
    )
    if err != nil {
        log.Fatal(err)
    }

    if err := engine.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer engine.Stop(ctx)

    log.Println("✅ Production server running")

    engine.Wait()
}
```

### 8.4 Testing Pattern

```go
func TestWorkflowExecution(t *testing.T) {
    ctx := t.Context()

    // Build test workflow
    testWorkflow, _ := workflow.New(ctx, "test-workflow",
        workflow.WithName("Test Workflow"),
        workflow.WithTasks([]*task.Config{...}),
    )

    testAgent, _ := agent.New(ctx, "test-agent",
        agent.WithModel("gpt-4"),
        agent.WithInstructions("Test assistant"),
    )

    // Create test engine
    engine, err := compozy.New(ctx,
        compozy.WithMode(compozy.ModeStandalone),
        compozy.WithStandaloneTemporal(&compozy.StandaloneTemporalConfig{
            DatabaseFile: ":memory:",
        }),
        compozy.WithWorkflow(testWorkflow),
        compozy.WithAgent(testAgent),
    )
    require.NoError(t, err)

    require.NoError(t, engine.Start(ctx))
    defer engine.Stop(ctx)

    // Execute test
    result, err := engine.ExecuteWorkflowSync(ctx, "test-workflow",
        &compozy.ExecuteSyncRequest{
            Input: map[string]any{"test": "data"},
        },
    )
    require.NoError(t, err)
    assert.Equal(t, "expected", result.Output["result"])
}
```

### 8.5 Streaming Execution

```go
func main() {
    ctx := context.Background()

    engine, _ := compozy.New(ctx, ...)
    engine.Start(ctx)
    defer engine.Stop(ctx)

    // Stream workflow execution
    stream, err := engine.ExecuteWorkflowStream(ctx, "chat-workflow",
        &compozy.ExecuteRequest{
            Input: map[string]any{"message": "Hello"},
        },
        &client.StreamOptions{
            Events:       []string{"task.completed", "agent.message"},
            PollInterval: 100 * time.Millisecond,
        },
    )
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    // Process events
    for {
        select {
        case event := <-stream.Events():
            fmt.Printf("[%s] %s\n", event.Type, string(event.Data))
        case err := <-stream.Errors():
            if err != nil {
                log.Printf("Stream error: %v", err)
            }
            return
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 9. Test Strategy

### 9.1 Unit Tests

**Constructor Tests (`constructor_test.go`):**

- Functional options work correctly
- Default values set properly
- Validation catches invalid configurations
- Context required enforcement

**Engine Tests (`engine_test.go`):**

- Lifecycle (Start/Stop/Wait) works correctly
- Resource registration succeeds
- Duplicate detection works
- Resource retrieval works

**Mode Tests (`mode_test.go`):**

- Mode determination logic
- Standalone validation (required fields)
- Distributed validation (required fields)
- Mode-specific defaults

**Validation Tests (`validation_test.go`):**

- Reference validation catches missing refs
- Circular dependency detection
- Dependency graph construction
- Validation report format

### 9.2 Integration Tests

**Standalone Mode Integration:**

- Full lifecycle with embedded Temporal
- Embedded Redis functionality
- Resource loading and registration
- Workflow execution (sync/async/stream)
- Proper cleanup on shutdown

**Distributed Mode Integration:**

- Connection to external services
- Resource persistence to Redis
- Workflow execution via external Temporal
- Error handling for unavailable services

**Client Integration:**

- Engine delegates to client correctly
- Request/response conversion works
- Streaming works end-to-end
- Error propagation correct

### 9.3 Code Generation Tests

**Generator Tests:**

- Resource spec parsing
- Option generation
- Execution method generation
- Loading method generation
- Registration method generation
- Generated code compiles
- Generated code has proper imports

### 9.4 Coverage Goals

- Unit tests: **85%+** coverage on core logic
- Integration tests: All critical paths covered
- Generator tests: **90%+** coverage
- E2E tests: Major use cases (standalone, distributed, hybrid)

---

## 10. Implementation Rules & Standards

### 10.1 Follow Project Standards

**Critical Rules:**

- See `.cursor/rules/go-coding-standards.mdc`
- See `.cursor/rules/architecture.mdc`
- See `.cursor/rules/test-standards.mdc`
- See `.cursor/rules/no-linebreaks.mdc`

**Key Requirements:**

- Use `logger.FromContext(ctx)` - NEVER pass logger as parameter
- Use `config.FromContext(ctx)` - NEVER use global config
- NEVER use `context.Background()` in runtime code
- Functions < 50 lines (break into smaller functions)
- No magic numbers - use constants or config
- Use `t.Context()` in tests, not `context.Background()`

### 10.2 SDK2 Pattern Compliance

**Functional Options:**

- All configuration via functional options
- Options are composable and order-independent
- Validation happens in constructor, not in options
- Return `Option func(*config)` from option functions

**Constructor Pattern:**

```go
func New(ctx context.Context, opts ...Option) (*T, error) {
    if ctx == nil {
        return nil, fmt.Errorf("context required")
    }
    cfg := defaultConfig()
    for _, opt := range opts {
        if opt != nil {
            opt(cfg)
        }
    }
    return build(ctx, cfg)
}
```

### 10.3 Context Propagation

**Mandatory:**

- All I/O operations accept `context.Context`
- Propagate context through call chains
- Handle context cancellation properly
- Use `context.WithTimeout` for operations with deadlines

### 10.4 Error Handling

**Patterns:**

- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Return errors immediately (fail fast)
- Use typed errors for different error classes
- Include actionable information in error messages

### 10.5 Resource Management

**Lifecycle:**

- Always close/cleanup in defer
- Handle errors from cleanup operations
- Use `errgroup` for concurrent operations
- Avoid goroutine leaks

**Concurrency:**

- Use `sync.WaitGroup.Go()` (Go 1.25+)
- Protect shared state with mutexes
- Document goroutine ownership

---

## 11. API Contracts

### 11.1 Constructor Contract

**New() Behavior:**

- Requires non-nil context
- Applies options in order
- Validates configuration
- Returns error if validation fails
- Does NOT start engine (explicit Start() required)

**Option Functions:**

- Nil-safe (check config != nil)
- Composable (order doesn't matter for most options)
- Idempotent where possible
- For slices: append, don't replace

### 11.2 Lifecycle Contract

**Start():**

- Can only be called once (calling twice = error)
- Starts embedded services if standalone mode
- Creates HTTP client pointing to server
- Blocks until server is ready
- Returns error if startup fails

**Stop():**

- Gracefully shuts down all services
- Waits for in-flight requests (respects context timeout)
- Stops embedded services if standalone
- Can be called multiple times (idempotent)
- Always returns last error if any

**Wait():**

- Blocks until engine stops
- Safe to call multiple times
- Used for keeping servers running

### 11.3 Execution Contract

**All Execute Methods:**

- Require non-nil context
- Require non-empty resource ID
- Require non-nil request
- Delegate to internal client
- Convert request/response types
- Return error immediately on failure

**ExecuteWorkflow/Task/Agent (Async):**

- Returns immediately with execution handle
- Workflow starts in background
- Returns ExecuteResponse with ExecID

**ExecuteWorkflowSync/TaskSync/AgentSync:**

- Blocks until completion or timeout
- Returns final output
- Respects context cancellation

**ExecuteWorkflowStream/TaskStream/AgentStream:**

- Returns immediately with StreamSession
- Events delivered via channel
- Errors delivered via separate channel
- Must call Close() to release resources

### 11.4 Resource Loading Contract

**Load\*() Methods:**

- Parse YAML file
- Validate structure
- Call corresponding Register\*() method
- Return error if file invalid or registration fails

**Load\*FromDir() Methods:**

- Discover all \*.yaml files in directory
- Load each file via Load\*()
- Continue on errors (collect and return)
- Non-recursive (only immediate directory)

### 11.5 Resource Registration Contract

**Register\*() Methods:**

- Accept validated config from sdk constructors
- Persist to resource store
- Validate references (soft validation)
- Detect duplicates (return error)
- Can be called before or after Start()

### 11.6 Validation Contract

**ValidateReferences():**

- Can be called anytime (before or after Start)
- Non-blocking
- Returns detailed report
- Validates all cross-references
- Detects circular dependencies
- Builds dependency graph

---

## 12. Migration from Legacy SDK

### 12.1 API Mapping

| Legacy SDK (`sdk/compozy`)           | New SDK (`sdk/compozy`)                                                           |
| ------------------------------------ | ---------------------------------------------------------------------------------- |
| `compozy.New(id).WithHost().Build()` | `compozy.New(ctx, compozy.WithHost())`                                             |
| `instance.Start()`                   | `engine.Start(ctx)`                                                                |
| `instance.Stop()`                    | `engine.Stop(ctx)`                                                                 |
| `instance.RegisterWorkflow(cfg)`     | `engine.RegisterWorkflow(cfg)` OR<br>`compozy.New(ctx, compozy.WithWorkflow(cfg))` |
| `instance.ExecuteWorkflow()`         | `engine.ExecuteWorkflowSync()`                                                     |
| `instance.ValidateReferences()`      | `engine.ValidateReferences()`                                                      |

### 12.2 Breaking Changes

**Context Required:**

- Constructor now requires context: `New(ctx, ...)`
- Start/Stop now require context
- All execution methods already required context (no change)

**SDK2 Constructors:**

- Must use sdk package constructors: `workflow.New()`, `agent.New()`
- Configs come from sdk packages, not engine packages

**Unified Request Types:**

- Single `ExecuteRequest` for all async execution
- Single `ExecuteSyncRequest` for all sync execution
- No separate `WorkflowExecuteRequest`, `TaskExecuteRequest`, etc.

**Client Integration:**

- Execution via internal client (HTTP transport)
- Streaming returns `*client.StreamSession` type

---

## 13. Implementation Timeline

### Phase 1: Foundation (2 days)

- [ ] Create package structure
- [ ] Implement type definitions (`types.go`)
- [ ] Implement mode types and validation
- [ ] Write constructor with functional options
- [ ] Unit tests for constructor and modes

### Phase 2: Code Generation (2 days)

- [ ] Extend codegen infrastructure from sdk/internal/codegen
- [ ] Create resource specifications
- [ ] Implement option generator
- [ ] Implement execution method generator
- [ ] Implement loading method generator
- [ ] Implement registration method generator
- [ ] Generate all methods and verify compilation

### Phase 3: Engine Core (2 days)

- [ ] Implement engine struct and lifecycle
- [ ] Implement Start/Stop/Wait
- [ ] Implement client integration
- [ ] Implement resource store selection
- [ ] Implement introspection methods
- [ ] Unit tests for lifecycle

### Phase 4: Mode Implementation (2 days)

- [ ] Implement standalone mode (embedded Temporal/Redis)
- [ ] Implement distributed mode (external connections)
- [ ] Mode validation and error handling
- [ ] Integration tests for both modes

### Phase 5: Resource Management (1 day)

- [ ] Implement YAML loading utilities
- [ ] Implement validation logic
- [ ] Implement dependency graph construction
- [ ] Resource management tests

### Phase 6: Testing & Documentation (2 days)

- [ ] Write integration tests
- [ ] Write E2E tests for all use cases
- [ ] Performance testing
- [ ] Write package documentation
- [ ] Create README with examples
- [ ] Create migration guide

**Total: 11 days (9 implementation + 2 testing/docs)**

---

## 14. Success Metrics

### 14.1 Code Quality

✅ **Test Coverage:** 85%+ on core logic  
✅ **Linting:** Zero golangci-lint warnings  
✅ **Documentation:** 100% godoc coverage on exported APIs  
✅ **Generated Code:** 80%+ of boilerplate auto-generated

### 14.2 Performance

✅ **Standalone Startup:** < 5 seconds on modern hardware  
✅ **Distributed Startup:** < 2 seconds (connection only)  
✅ **Resource Loading:** < 100ms per YAML file  
✅ **Validation:** < 500ms for 100 resources

### 14.3 Developer Experience

✅ **API Discoverability:** All methods auto-completable in IDE  
✅ **Error Messages:** Actionable with clear next steps  
✅ **Examples:** 10+ working examples in documentation  
✅ **Migration:** < 1 hour for typical legacy SDK usage

---

## 15. Open Questions & Decisions

### 15.1 YAML Auto-Loading

**Question:** Should `WithConfigFile()` auto-load all resources?

**Options:**

1. Load everything (workflows, agents, tools from project config)
2. Load only project metadata, require explicit Load\*() calls
3. Configurable behavior

**Decision:** Option 2 - Explicit is better. Users call `LoadWorkflowsFromDir()` etc.

### 15.2 Resource Store Selection

**Question:** How to select resource store?

**Current Logic:**

- Standalone: Memory store (or Redis if available)
- Distributed: Redis store (required)

**Decision:** Follow engine logic in `engine/infra/server/dependencies.go:365-376`

### 15.3 Default Mode

**Question:** What should be the default mode?

**Options:**

1. Standalone (easier getting started)
2. Distributed (production-ready)
3. Require explicit mode

**Decision:** Option 1 - Standalone default (matches use case priority)

---

## 16. References

### Project Standards

- `.cursor/rules/go-coding-standards.mdc` - Go coding standards
- `.cursor/rules/architecture.mdc` - Architecture patterns
- `.cursor/rules/test-standards.mdc` - Testing requirements
- `.cursor/rules/global-config.mdc` - Configuration management
- `.cursor/rules/logger-config.mdc` - Logger & context patterns
- `.cursor/rules/no-linebreaks.mdc` - Code formatting

### Related Specifications

- `tasks/prd-modes/_prd.md` - Operational modes PRD
- `tasks/prd-redis/_techspec.md` - Standalone mode tech spec
- `sdk/MIGRATION_GUIDE.md` - SDK v2 migration patterns
- `sdk/CODEGEN_COMPARISON.md` - Code generation approach

### Engine Documentation

- `engine/infra/server/README.md` - Server infrastructure
- `engine/resources/README.md` - Resource management
- `docs/content/docs/deployment/standalone-mode.mdx` - Standalone docs

---

## 17. Approval & Sign-off

**Technical Lead:** _Pending Review_  
**Architecture Review:** _Pending Review_  
**Implementation Start:** _TBD_  
**Target Completion:** _TBD_

---

**Document Version:** 2.0  
**Last Updated:** 2025-10-31  
**Status:** Draft - Ready for Review  
**Changes from v1.0:**

- Explicit API design (no implicit behavior)
- Unified request/response types
- SDK2 constructor pattern compliance
- Client integration clarified
- Code generation strategy refined
