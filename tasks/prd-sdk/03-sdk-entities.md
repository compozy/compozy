# SDK Entities: Complete API Reference

**Date:** 2025-01-25
**Version:** 2.0.0
**Estimated Reading Time:** 40 minutes

---

## Overview

This document provides the complete API reference for all builders in the Compozy GO SDK.

**Key Updates in sdk.0:**
- ✅ All `Build()` methods require `context.Context`
- ✅ 9 task types (matching engine)
- ✅ Full memory system features
- ✅ Complete MCP configuration
- ✅ Native tools integration
- ✅ Error accumulation pattern
- ✅ All missing builders added

**Entity Categories:**
1. Project Configuration (`sdk/project`)
2. Model Configuration (`sdk/model`)
3. Workflow Construction (`sdk/workflow`)
4. Agent Definition (`sdk/agent` + `ActionBuilder`)
5. Task Creation (`sdk/task` - 9 types)
6. Knowledge System (`sdk/knowledge` - 5 builders)
7. Memory System (`sdk/memory` - 2 builders, full features)
8. MCP Integration (`sdk/mcp` - full config)
9. Runtime Configuration (`sdk/runtime` + native tools)
10. Tool Definition (`sdk/tool`)
11. Schema Validation (`sdk/schema` - 2 builders)
12. Schedule Configuration (`sdk/schedule`)
13. Monitoring Configuration (`sdk/monitoring`)
14. Compozy Embedded Engine (`sdk/compozy`)
15. Signal System (unified) (`sdk/task.Signal`)
16. Client SDK (`sdk/client`)

---

## 1. Project Configuration

### Package: `sdk/project`

**Purpose:** Top-level project configuration containing all resources

**Go SDK API:**
```go
package project

import (
    "context"
    
    "github.com/compozy/compozy/engine/project"
)

type Builder struct {
    config *project.Config
    errors []error
}

// Constructor
func New(name string) *Builder

// Core metadata
func (b *Builder) WithVersion(version string) *Builder
func (b *Builder) WithDescription(desc string) *Builder
func (b *Builder) WithAuthor(name, email, org string) *Builder

// Resource registration
func (b *Builder) AddModel(model *core.ProviderConfig) *Builder
func (b *Builder) AddEmbedder(embedder *knowledge.EmbedderConfig) *Builder
func (b *Builder) AddVectorDB(vectorDB *knowledge.VectorDBConfig) *Builder
func (b *Builder) AddKnowledgeBase(kb *knowledge.BaseConfig) *Builder
func (b *Builder) AddMemory(mem *memory.Config) *Builder
func (b *Builder) AddMCP(mcp *mcp.Config) *Builder
func (b *Builder) AddWorkflow(wf *workflow.Config) *Builder
func (b *Builder) AddAgent(agent *agent.Config) *Builder
func (b *Builder) AddTool(tool *tool.Config) *Builder
func (b *Builder) AddSchema(schema *schema.Schema) *Builder
func (b *Builder) AddSchedule(schedule *schedule.Config) *Builder

// Runtime and monitoring
func (b *Builder) WithRuntime(runtime *runtime.Config) *Builder
func (b *Builder) WithMonitoring(mon *monitoring.Config) *Builder

// AutoLoad (for hybrid SDK+YAML projects)
func (b *Builder) WithAutoLoad(enabled bool, include, exclude []string) *Builder

// Build with context (MANDATORY)
func (b *Builder) Build(ctx context.Context) (*project.Config, error)
```

**Usage Example:**
```go
ctx := context.Background()

proj, err := project.New("my-ai-project").
    WithVersion("1.0.0").
    WithDescription("AI assistant with knowledge and memory").
    WithAuthor("John Doe", "john@example.com", "ACME Corp").
    AddModel(model).
    AddWorkflow(wf).
    AddMemory(memConfig).
    Build(ctx)  // ✅ Context required

if err != nil {
    // Handle BuildError with accumulated errors
    log.Fatal(err)
}
```

**Validation:**
- Name is required (alphanumeric + hyphens)
- Version must be valid semver (if provided)
- At least one workflow required
- All resource IDs must be unique

---

## 2. Model Configuration

### Package: `sdk/model`

**Go SDK API:**
```go
package model

import (
    "context"
    
    "github.com/compozy/compozy/engine/core"
)

type Builder struct {
    config *core.ProviderConfig
    errors []error
}

// Constructor
func New(provider, model string) *Builder

// API configuration
func (b *Builder) WithAPIKey(key string) *Builder
func (b *Builder) WithAPIURL(url string) *Builder

// Model parameters
func (b *Builder) WithDefault(isDefault bool) *Builder
func (b *Builder) WithTemperature(temp float64) *Builder
func (b *Builder) WithMaxTokens(max int) *Builder
func (b *Builder) WithTopP(topP float64) *Builder
func (b *Builder) WithFrequencyPenalty(penalty float64) *Builder
func (b *Builder) WithPresencePenalty(penalty float64) *Builder
func (b *Builder) WithParams(params map[string]interface{}) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*core.ProviderConfig, error)
```

**Usage Example:**
```go
model, err := model.New("openai", "gpt-4-turbo").
    WithAPIKey(os.Getenv("OPENAI_API_KEY")).
    WithDefault(true).
    WithTemperature(0.7).
    WithMaxTokens(4000).
    Build(ctx)
```

**Supported Providers:**
- `openai` - OpenAI (GPT-4, GPT-4o, GPT-3.5, etc.)
- `anthropic` - Anthropic (Claude models)
- `google` - Google (Gemini models)
- `groq` - Groq (fast inference)
- `ollama` - Ollama (local models)

---

## 3. Workflow Construction

### Package: `sdk/workflow`

**Go SDK API:**
```go
package workflow

import (
    "context"
    
    "github.com/compozy/compozy/engine/workflow"
)

type Builder struct {
    config *workflow.Config
    errors []error
}

// Constructor
func New(id string) *Builder

// Core configuration
func (b *Builder) WithDescription(desc string) *Builder

// Resource registration
func (b *Builder) AddAgent(agent *agent.Config) *Builder
func (b *Builder) AddTask(task *task.Config) *Builder
func (b *Builder) AddTool(tool *tool.Config) *Builder
func (b *Builder) AddSchema(schema *schema.Schema) *Builder

// Input/output configuration
func (b *Builder) WithInput(schema *schema.Schema) *Builder
func (b *Builder) WithOutputs(outputs map[string]string) *Builder

// Scheduling
func (b *Builder) AddSchedule(schedule *schedule.Config) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*workflow.Config, error)

// FromYAML helper (for hybrid projects)
func FromYAML(wf *workflow.Config) *Builder
```

**Usage Example:**
```go
wf, err := workflow.New("my-workflow").
    WithDescription("Q&A with knowledge base").
    AddAgent(agent).
    AddTask(answerTask).
    WithOutputs(map[string]string{
        "answer": "{{ .tasks.answer.output }}",
    }).
    Build(ctx)
```

---

## 4. Agent Definition

### Package: `sdk/agent`

**Go SDK API:**
```go
package agent

import (
    "context"
    
    "github.com/compozy/compozy/engine/agent"
)

// Agent Builder
type Builder struct {
    config *agent.Config
    errors []error
}

// Constructor
func New(id string) *Builder

// Core configuration
func (b *Builder) WithModel(provider, model string) *Builder
func (b *Builder) WithModelRef(modelID string) *Builder  // Reference to project model
func (b *Builder) WithInstructions(instructions string) *Builder

// Knowledge and memory
func (b *Builder) WithKnowledge(binding *knowledge.BindingConfig) *Builder
func (b *Builder) WithMemory(ref *memory.ReferenceConfig) *Builder

// Actions and tools
func (b *Builder) AddAction(action *agent.ActionConfig) *Builder
func (b *Builder) AddTool(toolID string) *Builder

// MCP servers
func (b *Builder) AddMCP(mcpID string) *Builder

// Advanced configuration
func (b *Builder) WithConfig(config map[string]interface{}) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*agent.Config, error)


// Action Builder
type ActionBuilder struct {
    config *agent.ActionConfig
    errors []error
}

// Constructor
func NewAction(id string) *ActionBuilder

// Core configuration
func (a *ActionBuilder) WithPrompt(prompt string) *ActionBuilder
func (a *ActionBuilder) WithName(name string) *ActionBuilder
func (a *ActionBuilder) WithDescription(desc string) *ActionBuilder

// Output schema
func (a *ActionBuilder) WithOutput(output *schema.Schema) *ActionBuilder

// Tools per action
func (a *ActionBuilder) AddTool(toolID string) *ActionBuilder

// Transitions (success/error handling)
func (a *ActionBuilder) WithSuccessTransition(taskID string) *ActionBuilder
func (a *ActionBuilder) WithErrorTransition(taskID string) *ActionBuilder

// Retry configuration
func (a *ActionBuilder) WithRetry(maxAttempts int, backoff time.Duration) *ActionBuilder

// Timeout
func (a *ActionBuilder) WithTimeout(timeout time.Duration) *ActionBuilder

// Build with context
func (a *ActionBuilder) Build(ctx context.Context) (*agent.ActionConfig, error)
```

**Usage Example:**
```go
// Define action with full configuration
action, err := agent.NewAction("answer").
    WithPrompt("Answer the question: {{ .input.question }}").
    WithOutput(schema.NewObject().
        AddProperty("answer", schema.NewString().Build(ctx)).
        Build(ctx)).
    AddTool("search").
    WithTimeout(30 * time.Second).
    Build(ctx)

// Build agent with action
agent, err := agent.New("assistant").
    WithModel("openai", "gpt-4").
    WithInstructions("You are a helpful assistant.").
    WithKnowledge(knowledgeBinding).
    WithMemory(memoryRef).
    AddAction(action).
    AddTool("calculator").
    Build(ctx)
```

---

## 5. Task Creation (9 Types)

### Package: `sdk/task`

**Task Types (matching engine):**
```go
const (
    TaskTypeBasic      task.Type = "basic"       // Single agent/tool execution
    TaskTypeParallel   task.Type = "parallel"    // Parallel task execution
    TaskTypeCollection task.Type = "collection"  // Iterate over collection
    TaskTypeRouter     task.Type = "router"      // Conditional routing
    TaskTypeWait       task.Type = "wait"        // Wait for duration/condition
    TaskTypeAggregate  task.Type = "aggregate"   // Aggregate results
    TaskTypeComposite  task.Type = "composite"   // Nested workflow
    TaskTypeSignal     task.Type = "signal"      // Signal send/wait
    TaskTypeMemory     task.Type = "memory"      // Memory operations
)
```

### 5.1 Basic Task

```go
// BasicBuilder creates single-step agent or tool tasks
type BasicBuilder struct {
    config *task.Config
    errors []error
}

func NewBasic(id string) *BasicBuilder

// Agent execution
func (b *BasicBuilder) WithAgent(agentID string) *BasicBuilder
func (b *BasicBuilder) WithAction(actionID string) *BasicBuilder

// Tool execution
func (b *BasicBuilder) WithTool(toolID string) *BasicBuilder

// Input/output
func (b *BasicBuilder) WithInput(input map[string]string) *BasicBuilder
func (b *BasicBuilder) WithOutput(output string) *BasicBuilder

// Control flow
func (b *BasicBuilder) WithCondition(condition string) *BasicBuilder
func (b *BasicBuilder) WithFinal(isFinal bool) *BasicBuilder

// Build with context
func (b *BasicBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.2 Parallel Task

```go
// ParallelBuilder executes multiple tasks concurrently
type ParallelBuilder struct {
    config *task.Config
    errors []error
}

func NewParallel(id string) *ParallelBuilder

// Add tasks to execute in parallel
func (b *ParallelBuilder) AddTask(taskID string) *ParallelBuilder

// Wait for all or first completion
func (b *ParallelBuilder) WithWaitAll(waitAll bool) *ParallelBuilder

// Build with context
func (b *ParallelBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.3 Collection Task

```go
// CollectionBuilder iterates over a collection
type CollectionBuilder struct {
    config *task.Config
    errors []error
}

func NewCollection(id string) *CollectionBuilder

// Collection source (e.g., "{{ .input.items }}")
func (b *CollectionBuilder) WithCollection(collection string) *CollectionBuilder

// Task to execute for each item
func (b *CollectionBuilder) WithTask(taskID string) *CollectionBuilder

// Item variable name (default: "item")
func (b *CollectionBuilder) WithItemVar(varName string) *CollectionBuilder

// Build with context
func (b *CollectionBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.4 Router Task

```go
// RouterBuilder performs conditional routing (switch logic)
type RouterBuilder struct {
    config *task.Config
    errors []error
}

func NewRouter(id string) *RouterBuilder

// Condition to evaluate
func (b *RouterBuilder) WithCondition(condition string) *RouterBuilder

// Add conditional routes
func (b *RouterBuilder) AddRoute(condition string, taskID string) *RouterBuilder

// Default route (if no conditions match)
func (b *RouterBuilder) WithDefault(taskID string) *RouterBuilder

// Build with context
func (b *RouterBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.5 Wait Task

```go
// WaitBuilder waits for duration or condition
type WaitBuilder struct {
    config *task.Config
    errors []error
}

func NewWait(id string) *WaitBuilder

// Wait for fixed duration
func (b *WaitBuilder) WithDuration(duration time.Duration) *WaitBuilder

// Wait until condition is true
func (b *WaitBuilder) WithCondition(condition string) *WaitBuilder

// Maximum wait time
func (b *WaitBuilder) WithTimeout(timeout time.Duration) *WaitBuilder

// Build with context
func (b *WaitBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.6 Aggregate Task

```go
// AggregateBuilder aggregates results from multiple tasks
type AggregateBuilder struct {
    config *task.Config
    errors []error
}

func NewAggregate(id string) *AggregateBuilder

// Tasks to aggregate
func (b *AggregateBuilder) AddTask(taskID string) *AggregateBuilder

// Aggregation strategy
func (b *AggregateBuilder) WithStrategy(strategy string) *AggregateBuilder  // "concat", "merge", "custom"

// Custom aggregation function
func (b *AggregateBuilder) WithFunction(fn string) *AggregateBuilder

// Build with context
func (b *AggregateBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.7 Composite Task

```go
// CompositeBuilder nests a workflow as a task
type CompositeBuilder struct {
    config *task.Config
    errors []error
}

func NewComposite(id string) *CompositeBuilder

// Workflow to execute
func (b *CompositeBuilder) WithWorkflow(workflowID string) *CompositeBuilder

// Input mapping
func (b *CompositeBuilder) WithInput(input map[string]string) *CompositeBuilder

// Build with context
func (b *CompositeBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.8 Signal Task

```go
// SignalBuilder sends or waits for signals (unified)
type SignalBuilder struct {
    config *task.Config
    errors []error
}

func NewSignal(id string) *SignalBuilder

// Signal send operation
func (b *SignalBuilder) Send(signalName string, payload map[string]interface{}) *SignalBuilder

// Signal wait operation
func (b *SignalBuilder) Wait(signalName string) *SignalBuilder

// Timeout for wait
func (b *SignalBuilder) WithTimeout(timeout time.Duration) *SignalBuilder

// Build with context
func (b *SignalBuilder) Build(ctx context.Context) (*task.Config, error)
```

### 5.9 Memory Task

```go
// MemoryTaskBuilder performs memory operations
type MemoryTaskBuilder struct {
    config *task.Config
    errors []error
}

func NewMemoryTask(id string) *MemoryTaskBuilder

// Memory operations
func (b *MemoryTaskBuilder) WithOperation(op string) *MemoryTaskBuilder  // "read", "append", "clear"

// Memory reference
func (b *MemoryTaskBuilder) WithMemory(memoryID string) *MemoryTaskBuilder

// Content to append
func (b *MemoryTaskBuilder) WithContent(content string) *MemoryTaskBuilder

// Build with context
func (b *MemoryTaskBuilder) Build(ctx context.Context) (*task.Config, error)
```

---

## 6. Knowledge System (5 Builders)

### 6.1 Embedder Configuration

```go
package knowledge

import (
    "context"
    
    "github.com/compozy/compozy/engine/knowledge"
)

type EmbedderBuilder struct {
    config *knowledge.EmbedderConfig
    errors []error
}

func NewEmbedder(id, provider, model string) *EmbedderBuilder

// API configuration
func (b *EmbedderBuilder) WithAPIKey(key string) *EmbedderBuilder

// Runtime configuration
func (b *EmbedderBuilder) WithDimension(dim int) *EmbedderBuilder
func (b *EmbedderBuilder) WithBatchSize(size int) *EmbedderBuilder
func (b *EmbedderBuilder) WithMaxConcurrentWorkers(max int) *EmbedderBuilder

// Build with context
func (b *EmbedderBuilder) Build(ctx context.Context) (*knowledge.EmbedderConfig, error)
```

### 6.2 Vector Database Configuration

```go
type VectorDBBuilder struct {
    config *knowledge.VectorDBConfig
    errors []error
}

func NewVectorDB(id string, dbType knowledge.VectorDBType) *VectorDBBuilder

// Connection configuration
func (b *VectorDBBuilder) WithDSN(dsn string) *VectorDBBuilder
func (b *VectorDBBuilder) WithPath(path string) *VectorDBBuilder
func (b *VectorDBBuilder) WithCollection(collection string) *VectorDBBuilder

// PGVector-specific configuration
func (b *VectorDBBuilder) WithPGVectorIndex(indexType string, lists int) *VectorDBBuilder
func (b *VectorDBBuilder) WithPGVectorPool(minConns, maxConns int32) *VectorDBBuilder

// Build with context
func (b *VectorDBBuilder) Build(ctx context.Context) (*knowledge.VectorDBConfig, error)
```

### 6.3 Source Configuration

```go
type SourceBuilder struct {
    config *knowledge.SourceConfig
    errors []error
}

// Constructors for different source types
func NewFileSource(path string) *SourceBuilder
func NewDirectorySource(paths ...string) *SourceBuilder
func NewURLSource(urls ...string) *SourceBuilder
func NewAPISource(provider string) *SourceBuilder

// Build with context
func (b *SourceBuilder) Build(ctx context.Context) (*knowledge.SourceConfig, error)
```

### 6.4 Knowledge Base Configuration

```go
type BaseBuilder struct {
    config *knowledge.BaseConfig
    errors []error
}

func NewBase(id string) *BaseBuilder

// Core configuration
func (b *BaseBuilder) WithDescription(desc string) *BaseBuilder
func (b *BaseBuilder) WithEmbedder(embedderID string) *BaseBuilder
func (b *BaseBuilder) WithVectorDB(vectorDBID string) *BaseBuilder

// Ingestion
func (b *BaseBuilder) AddSource(source *knowledge.SourceConfig) *BaseBuilder
func (b *BaseBuilder) WithChunking(strategy knowledge.ChunkStrategy, size, overlap int) *BaseBuilder
func (b *BaseBuilder) WithPreprocess(dedupe, removeHTML bool) *BaseBuilder
func (b *BaseBuilder) WithIngestMode(mode knowledge.IngestMode) *BaseBuilder

// Retrieval
func (b *BaseBuilder) WithRetrieval(topK int, minScore float64, maxTokens int) *BaseBuilder

// Build with context
func (b *BaseBuilder) Build(ctx context.Context) (*knowledge.BaseConfig, error)
```

### 6.5 Knowledge Binding (Agent Attachment)

```go
type BindingBuilder struct {
    config *knowledge.BindingConfig
    errors []error
}

func NewBinding(knowledgeBaseID string) *BindingBuilder

// Retrieval parameters
func (b *BindingBuilder) WithTopK(topK int) *BindingBuilder
func (b *BindingBuilder) WithMinScore(score float64) *BindingBuilder
func (b *BindingBuilder) WithMaxTokens(max int) *BindingBuilder

// Build with context
func (b *BindingBuilder) Build(ctx context.Context) (*knowledge.BindingConfig, error)
```

---

## 7. Memory System (2 Builders, Full Features)

### 7.1 Memory Configuration

```go
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

func New(id string) *ConfigBuilder

// Core configuration
func (b *ConfigBuilder) WithProvider(provider string) *ConfigBuilder
func (b *ConfigBuilder) WithModel(model string) *ConfigBuilder
func (b *ConfigBuilder) WithMaxTokens(max int) *ConfigBuilder

// Flush strategies
func (b *ConfigBuilder) WithFlushStrategy(strategy memory.FlushStrategy) *ConfigBuilder
func (b *ConfigBuilder) WithFIFOFlush(maxMessages int) *ConfigBuilder
func (b *ConfigBuilder) WithSummarizationFlush(provider, model string, maxTokens int) *ConfigBuilder

// Privacy and security
func (b *ConfigBuilder) WithPrivacy(privacy memory.PrivacyScope) *ConfigBuilder
func (b *ConfigBuilder) WithExpiration(duration time.Duration) *ConfigBuilder

// Persistence backend
func (b *ConfigBuilder) WithPersistence(backend memory.PersistenceBackend) *ConfigBuilder

// Token counting
func (b *ConfigBuilder) WithTokenCounter(provider, model string) *ConfigBuilder

// Distributed locking (for concurrent access)
func (b *ConfigBuilder) WithDistributedLocking(enabled bool) *ConfigBuilder

// Build with context
func (b *ConfigBuilder) Build(ctx context.Context) (*memory.Config, error)
```

### 7.2 Memory Reference (Agent Attachment)

```go
type ReferenceBuilder struct {
    config *memory.ReferenceConfig
    errors []error
}

func NewReference(memoryID string) *ReferenceBuilder

// Key template (e.g., "conversation-{{.conversation.id}}")
func (b *ReferenceBuilder) WithKey(keyTemplate string) *ReferenceBuilder

// Build with context
func (b *ReferenceBuilder) Build(ctx context.Context) (*memory.ReferenceConfig, error)
```

**Usage Example:**
```go
// Advanced memory configuration
memConfig, err := memory.New("customer-support").
    WithProvider("openai").
    WithModel("gpt-4o-mini").
    WithMaxTokens(2000).
    WithSummarizationFlush("openai", "gpt-4", 1000).
    WithPrivacy(memory.PrivacyUserScope).
    WithExpiration(24 * time.Hour).
    WithPersistence(memory.PersistenceRedis).
    WithDistributedLocking(true).
    Build(ctx)

// Agent memory reference
memRef, err := memory.NewReference("customer-support").
    WithKey("conversation-{{.conversation.id}}").
    Build(ctx)
```

---

## 8. MCP Integration (Full Configuration)

### Package: `sdk/mcp`

```go
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

func New(id string) *Builder

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

**Usage Examples:**

```go
// Stdio MCP with environment
mcpLocal, err := mcp.New("filesystem").
    WithCommand("mcp-server-filesystem").
    WithEnvVar("ROOT_DIR", "/data").
    WithStartTimeout(10 * time.Second).
    Build(ctx)

// Remote MCP with SSE transport and auth
mcpRemote, err := mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithTransport(mcpproxy.TransportSSE).
    WithHeader("Authorization", "Bearer {{.env.GITHUB_TOKEN}}").
    WithProto("2025-03-26").
    WithMaxSessions(10).
    Build(ctx)
```

---

## 15. Signal System (Unified)

### Package: `sdk/task` (SignalBuilder)

The Signal system is exposed via a unified SignalBuilder under `sdk/task`. It supports both send and wait modes.

```go
// sdk/task/signal.go
package task

type SignalMode string
const (
    SignalModeSend SignalMode = "send"
    SignalModeWait SignalMode = "wait"
)

type SignalBuilder struct {
    config *task.Config
    errors []error
}

func NewSignal(id string) *SignalBuilder
func (b *SignalBuilder) WithSignalID(id string) *SignalBuilder
func (b *SignalBuilder) WithMode(mode SignalMode) *SignalBuilder // send | wait
func (b *SignalBuilder) WithTimeout(d time.Duration) *SignalBuilder // wait mode
func (b *SignalBuilder) WithData(values map[string]interface{}) *SignalBuilder // send mode
func (b *SignalBuilder) OnSuccess(taskID string) *SignalBuilder
func (b *SignalBuilder) OnError(taskID string) *SignalBuilder
func (b *SignalBuilder) Build(ctx context.Context) (*task.Config, error)
```

---

## 16. Client SDK

### Package: `sdk/client`

Provides a simple HTTP client to interact with a running Compozy server (deploy projects, execute workflows, query status).

```go
package client

import (
    "context"
    "time"
)

type Builder struct {
    endpoint string
    apiKey   string
    timeout  time.Duration
    errors   []error
}

func New(endpoint string) *Builder
func (b *Builder) WithAPIKey(key string) *Builder
func (b *Builder) WithTimeout(d time.Duration) *Builder
func (b *Builder) Build(ctx context.Context) (*Client, error)

type Client struct { /* ... */ }

// Common operations (names illustrative for PRD)
func (c *Client) DeployProject(ctx context.Context, proj *project.Config) error
func (c *Client) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*ExecutionResult, error)
func (c *Client) GetWorkflowStatus(ctx context.Context, executionID string) (*WorkflowStatus, error)
```

Usage example appears in 01-executive-summary and 06-migration-guide.

---

## 9. Runtime Configuration (+ Native Tools)

### 9.1 Runtime Builder

```go
package runtime

import (
    "context"
    
    "github.com/compozy/compozy/engine/runtime"
)

type Builder struct {
    config *runtime.Config
    errors []error
}

// Constructors for different runtime types
func NewBun() *Builder
func NewNode() *Builder
func NewDeno() *Builder

// Entrypoint
func (b *Builder) WithEntrypoint(path string) *Builder

// Bun-specific permissions
func (b *Builder) WithBunPermissions(permissions ...string) *Builder

// Node-specific options
func (b *Builder) WithNodeOptions(options ...string) *Builder

// Deno-specific permissions
func (b *Builder) WithDenoPermissions(permissions ...string) *Builder

// Native tools integration
func (b *Builder) WithNativeTools(tools *runtime.NativeToolsConfig) *Builder

// Memory limits
func (b *Builder) WithMaxMemoryMB(mb int) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*runtime.Config, error)
```

### 9.2 Native Tools Builder

```go
type NativeToolsBuilder struct {
    config *runtime.NativeToolsConfig
}

func NewNativeTools() *NativeToolsBuilder

// Enable call_agents native tool
func (b *NativeToolsBuilder) WithCallAgents() *NativeToolsBuilder

// Enable call_workflows native tool
func (b *NativeToolsBuilder) WithCallWorkflows() *NativeToolsBuilder

// Build with context (kept for consistency with SDK patterns)
func (b *NativeToolsBuilder) Build(ctx context.Context) *runtime.NativeToolsConfig
```

**Usage Example:**
```go
runtime, err := runtime.NewBun().
    WithEntrypoint("./tools/main.ts").
    WithBunPermissions("--allow-read", "--allow-env").
    WithNativeTools(
        runtime.NewNativeTools().
            WithCallAgents().
            WithCallWorkflows().
            Build(ctx),
    ).
    WithMaxMemoryMB(512).
    Build(ctx)
```

---

## 10. Tool Definition

### Package: `sdk/tool`

```go
package tool

import (
    "context"
    
    "github.com/compozy/compozy/engine/tool"
)

type Builder struct {
    config *tool.Config
    errors []error
}

func New(id string) *Builder

// Core configuration
func (b *Builder) WithName(name string) *Builder
func (b *Builder) WithDescription(desc string) *Builder

// Runtime
func (b *Builder) WithRuntime(runtime string) *Builder  // "bun", "node", "deno"
func (b *Builder) WithCode(code string) *Builder

// Schemas
func (b *Builder) WithInput(schema *schema.Schema) *Builder
func (b *Builder) WithOutput(schema *schema.Schema) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*tool.Config, error)
```

---

## 11. Schema Validation (2 Builders)

### 11.1 Schema Builder

```go
package schema

import (
    "context"
    
    "github.com/compozy/compozy/engine/schema"
)

type Builder struct {
    schema schema.Schema
    errors []error
}

// Type constructors
func NewObject() *Builder
func NewString() *Builder
func NewNumber() *Builder
func NewInteger() *Builder
func NewBoolean() *Builder
func NewArray(itemType *Builder) *Builder

// Object properties
func (b *Builder) AddProperty(name string, prop *Builder) *Builder
func (b *Builder) RequireProperty(name string) *Builder

// String constraints
func (b *Builder) WithMinLength(min int) *Builder
func (b *Builder) WithMaxLength(max int) *Builder
func (b *Builder) WithPattern(pattern string) *Builder
func (b *Builder) WithEnum(values ...string) *Builder

// Number constraints
func (b *Builder) WithMinimum(min float64) *Builder
func (b *Builder) WithMaximum(max float64) *Builder

// Array constraints
func (b *Builder) WithMinItems(min int) *Builder
func (b *Builder) WithMaxItems(max int) *Builder

// Common
func (b *Builder) WithDefault(value interface{}) *Builder
func (b *Builder) WithDescription(desc string) *Builder

// Validation
func (b *Builder) ValidateSchema(ctx context.Context) error
func (b *Builder) TestAgainstSample(ctx context.Context, sample interface{}) error

// Schema references
func (b *Builder) WithRef(schemaID string) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*schema.Schema, error)
```

### 11.2 Property Builder

```go
type PropertyBuilder struct {
    property schema.Property
    errors   []error
}

func NewProperty(name string) *PropertyBuilder

// Type and constraints (same methods as Schema Builder)
func (b *PropertyBuilder) WithType(typ string) *PropertyBuilder
func (b *PropertyBuilder) WithDescription(desc string) *PropertyBuilder
func (b *PropertyBuilder) WithDefault(value interface{}) *PropertyBuilder
func (b *PropertyBuilder) Required() *PropertyBuilder

// Build with context
func (b *PropertyBuilder) Build(ctx context.Context) (*schema.Property, error)
```

**Usage Example:**
```go
// Complex object schema
outputSchema, err := schema.NewObject().
    AddProperty("answer", 
        schema.NewString().
            WithDescription("The answer to the question").
            WithMinLength(1).
            Build(ctx)).
    AddProperty("confidence", 
        schema.NewNumber().
            WithMinimum(0.0).
            WithMaximum(1.0).
            Build(ctx)).
    RequireProperty("answer").
    Build(ctx)
```

---

## 12. Schedule Configuration

### Package: `sdk/schedule`

```go
package schedule

import (
    "context"
    
    "github.com/compozy/compozy/engine/workflow/schedule"
)

type Builder struct {
    config *schedule.Config
    errors []error
}

func New(id string) *Builder

// Cron expression
func (b *Builder) WithCron(cron string) *Builder

// Workflow attachment
func (b *Builder) WithWorkflow(workflowID string) *Builder

// Input for scheduled executions
func (b *Builder) WithInput(input map[string]interface{}) *Builder

// Retry configuration
func (b *Builder) WithRetry(maxAttempts int, backoff time.Duration) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*schedule.Config, error)
```

**Usage Example:**
```go
schedule, err := schedule.New("daily-report").
    WithCron("0 9 * * *").  // Daily at 9 AM
    WithWorkflow("generate-report").
    WithInput(map[string]interface{}{
        "report_type": "daily",
    }).
    WithRetry(3, 5*time.Minute).
    Build(ctx)
```

---

## 13. Monitoring Configuration

### Package: `sdk/monitoring`

```go
package monitoring

import (
    "context"
    
    "github.com/compozy/compozy/engine/infra/monitoring"
)

type Builder struct {
    config *monitoring.Config
    errors []error
}

func New() *Builder

// Prometheus configuration
func (b *Builder) WithPrometheus(enabled bool) *Builder
func (b *Builder) WithPrometheusPort(port int) *Builder
func (b *Builder) WithPrometheusPath(path string) *Builder

// Custom metrics
func (b *Builder) AddCustomMetric(name, help string, metricType string) *Builder

// Distributed tracing
func (b *Builder) WithTracing(enabled bool, endpoint string) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*monitoring.Config, error)
```

---

## 14. Compozy Embedded Engine

### Package: `sdk/compozy`

**Main SDK package for embedding Compozy in Go applications.**

```go
package compozy

import (
    "context"
    
    "github.com/compozy/compozy/engine/infra/server"
)

// Compozy represents an embedded Compozy engine instance
type Compozy struct {
    server  *server.Server
    config  *config.Config
    project *project.Config
    ctx     context.Context
}

type Builder struct {
    project *project.Config
    errors  []error
    
    // Server configuration
    serverHost  string
    serverPort  int
    corsEnabled bool
    corsOrigins []string
    authEnabled bool
    
    // Infrastructure
    dbConnString string
    temporalHost string
    temporalNS   string
    redisURL     string
    
    // Runtime
    cwd        string
    configFile string
    envFile    string
    logLevel   string
}

// Constructor
func New(proj *project.Config) *Builder

// Server configuration
func (b *Builder) WithServerHost(host string) *Builder
func (b *Builder) WithServerPort(port int) *Builder
func (b *Builder) WithCORS(enabled bool, origins ...string) *Builder
func (b *Builder) WithAuth(enabled bool) *Builder

// Infrastructure (required)
func (b *Builder) WithDatabase(connString string) *Builder
func (b *Builder) WithTemporal(hostPort, namespace string) *Builder
func (b *Builder) WithRedis(url string) *Builder

// Runtime configuration
func (b *Builder) WithWorkingDirectory(cwd string) *Builder
func (b *Builder) WithConfigFile(path string) *Builder
func (b *Builder) WithEnvFile(path string) *Builder
func (b *Builder) WithLogLevel(level string) *Builder

// Build with context
func (b *Builder) Build(ctx context.Context) (*Compozy, error)

// Lifecycle methods
func (c *Compozy) Start() error
func (c *Compozy) Stop(ctx context.Context) error
func (c *Compozy) Wait() error

// Direct workflow execution
func (c *Compozy) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*ExecutionResult, error)

// Access internals
func (c *Compozy) Server() *server.Server
func (c *Compozy) Router() *gin.Engine
func (c *Compozy) Config() *config.Config
```

**Usage Example:**
```go
package main

import (
    "context"
    "log"
    
    "github.com/compozy/compozy/sdk/compozy"
    "github.com/compozy/compozy/sdk/project"
)

func main() {
    ctx := context.Background()
    
    // Build project with SDK
    proj, _ := project.New("my-app").
        AddWorkflow(wf).
        Build(ctx)
    
    // Embed Compozy engine
    app, err := compozy.New(proj).
        WithServerPort(8080).
        WithDatabase("postgres://localhost/myapp").
        WithTemporal("localhost:7233", "default").
        WithRedis("redis://localhost:6379").
        Build(ctx)
    
    if err != nil {
        log.Fatal(err)
    }
    
    // Start server (blocking)
    log.Println("Starting Compozy on :8080...")
    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

---

## Summary

### Complete Builder Coverage

| Package | Builder Types | Context Required | Status |
|---------|--------------|------------------|--------|
| `sdk/project` | 1 (Project) | ✅ Yes | ✅ Complete |
| `sdk/model` | 1 (Model) | ✅ Yes | ✅ Complete |
| `sdk/workflow` | 1 (Workflow) | ✅ Yes | ✅ Complete |
| `sdk/agent` | 2 (Agent, Action) | ✅ Yes | ✅ Complete |
| `sdk/task` | 9 (All types) | ✅ Yes | ✅ Complete |
| `sdk/knowledge` | 5 (Embedder, VectorDB, Source, Base, Binding) | ✅ Yes | ✅ Complete |
| `sdk/memory` | 2 (Config, Reference) | ✅ Yes | ✅ Complete |
| `sdk/mcp` | 1 (MCP) | ✅ Yes | ✅ Complete |
| `sdk/runtime` | 2 (Runtime, NativeTools) | ✅ Yes | ✅ Complete |
| `sdk/tool` | 1 (Tool) | ✅ Yes | ✅ Complete |
| `sdk/schema` | 2 (Schema, Property) | ✅ Yes | ✅ Complete |
| `sdk/schedule` | 1 (Schedule) | ✅ Yes | ✅ Complete |
| `sdk/monitoring` | 1 (Monitoring) | ✅ Yes | ✅ Complete |
| `sdk/compozy` | 1 (Compozy) | ✅ Yes | ✅ Complete |

**Total:** 30 builder types across 14 packages

### Key Improvements in sdk.0

1. ✅ **Context-first architecture** - All `Build()` methods require `context.Context`
2. ✅ **9 task types** - Complete engine task type coverage (not 6)
3. ✅ **Full memory system** - Flush strategies, privacy, distributed locking
4. ✅ **Complete MCP configuration** - Headers, transport, protocol, sessions
5. ✅ **Native tools integration** - `call_agents`, `call_workflows`
6. ✅ **Error accumulation** - BuildError aggregates multiple errors
7. ✅ **ActionBuilder** - Full action configuration with tools, transitions, retry
8. ✅ **Schema validation** - Compile and test schemas before runtime
9. ✅ **Source builder** - File, directory, URL, API sources
10. ✅ **Deno runtime support** - Bun, Node, and Deno runtimes
11. ✅ **Monitoring configuration** - Prometheus, custom metrics, tracing
12. ✅ **Builder immutability** - Deep cloning for independent configs

---

**End of SDK Entities Document**

**Status:** ✅ Complete (All P0, P1 issues addressed)
**Next Document:** 04-implementation-plan.md
