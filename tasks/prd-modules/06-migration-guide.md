# Migration Guide: YAML → Go SDK

**Date:** 2025-01-25
**Version:** 1.0.0
**Estimated Reading Time:** 15 minutes

---

## Overview

This guide helps you migrate from YAML configuration to the Compozy v2 Go SDK.

**Important:** YAML continues to work! Migration is **optional** and can be done gradually.

---

## Migration Strategies

### Strategy 1: Keep YAML (No Migration)
✅ **Recommended if:** Your workflows are stable and YAML works well for you

No changes needed. YAML will continue to work indefinitely.

### Strategy 2: Hybrid Approach
✅ **Recommended if:** You want type safety for new features while keeping existing YAML

Use YAML for existing workflows, Go SDK for new ones.

### Strategy 3: Full Migration
✅ **Recommended if:** You want full type safety and programmatic control

Convert all YAML to Go SDK over time.

---

## Quick Reference: YAML → Go

| YAML Concept | Go SDK Package | Example |
|--------------|----------------|---------|
| `name:` | `project.New()` | `project.New("my-project")` |
| `models:` | `model.New()` | `model.New("openai", "gpt-4")` |
| `workflows:` | `workflow.New()` | `workflow.New("my-workflow")` |
| `agents:` | `agent.New()` | `agent.New("assistant")` |
| `tasks:` | `task.NewBasic()` | `task.NewBasic("process")` |
| `knowledge_bases:` | `knowledge.NewBase()` | `knowledge.NewBase("docs")` |
| `memories:` | `memory.New()` | `memory.New("conversation")` |
| `mcps:` | `mcp.New()` | `mcp.New("github")` |
| `runtime:` | `runtime.New()` | `runtime.New("bun")` |

---

## Context Setup

All SDK builders require a context. In application code, inherit the request or parent context and attach logger and configuration using the standard Compozy helpers:

```go
import (
    "context"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/pkg/config"
)

var ctx context.Context

// Example setup (adjust for your app/server):
ctx = context.Background()               // Or r.Context() inside handlers
ctx = logger.WithLogger(ctx, logger.New())
ctx = config.WithConfig(ctx, config.Load())
```

For brevity, all examples below pass `ctx` to `Build(ctx)` without repeating the setup.

---

## Complete Migration Examples

### Example 1: Simple Project

**Before (YAML):**
```yaml
# compozy.yaml
name: simple-demo
version: 1.0.0
description: Simple demo project

models:
  - provider: openai
    model: gpt-4
    api_key: "{{ .env.OPENAI_API_KEY }}"
    default: true

workflows:
  - source: ./workflow.yaml
```

**After (Go SDK):**
```go
package main

import (
    "os"
    "github.com/compozy/compozy/v2/project"
    "github.com/compozy/compozy/v2/model"
)

func main() {
    proj, _ := project.New("simple-demo").
        WithVersion("1.0.0").
        WithDescription("Simple demo project").
        AddModel(
            model.New("openai", "gpt-4").
                WithAPIKey(os.Getenv("OPENAI_API_KEY")).
                WithDefault(true).
                Build(ctx),
        ).
        AddWorkflow(wf). // See workflow example below
        Build(ctx)
}
```

### Example 2: Workflow with Agent

**Before (YAML):**
```yaml
# workflow.yaml
id: greeting
description: Simple greeting workflow

agents:
  - id: assistant
    model: openai:gpt-4
    instructions: You are a helpful assistant.
    actions:
      - id: greet
        prompt: "Greet: {{ .input.name }}"
        output:
          type: object
          properties:
            greeting:
              type: string

tasks:
  - id: greet_user
    agent: assistant
    action: greet
    final: true
```

**After (Go SDK):**
```go
wf, _ := workflow.New("greeting").
    WithDescription("Simple greeting workflow").
    AddAgent(
        agent.New("assistant").
            WithModel("openai", "gpt-4").
            WithInstructions("You are a helpful assistant.").
            AddAction(
                agent.NewAction("greet").
                    WithPrompt("Greet: {{ .input.name }}").
                    WithOutput(
                        agent.NewObjectOutput().
                            AddProperty("greeting", agent.NewStringProperty()).
                            Build(ctx),
                    ).
                    Build(ctx),
            ).
            Build(ctx),
    ).
    AddTask(
        task.NewBasic("greet_user").
            WithAgent("assistant").
            WithAction("greet").
            Final().
            Build(ctx),
    ).
    Build(ctx)
```

### Example 3: Knowledge Base (RAG)

**Before (YAML):**
```yaml
# compozy.yaml
embedders:
  - id: openai_emb
    provider: openai
    model: text-embedding-3-small
    api_key: "{{ .env.OPENAI_API_KEY }}"
    config:
      dimension: 1536

vector_dbs:
  - id: pgvector
    type: pgvector
    config:
      dsn: "postgresql://localhost/vectors"
      dimension: 1536

knowledge_bases:
  - id: docs
    embedder: openai_emb
    vector_db: pgvector
    sources:
      - type: markdown_glob
        path: "docs/**/*.md"
    chunking:
      strategy: recursive_text_splitter
      size: 512
      overlap: 64
    retrieval:
      top_k: 5
```

**After (Go SDK):**
```go
import (
    "github.com/compozy/compozy/v2/knowledge"
)

// Embedder
embedder, _ := knowledge.NewEmbedder("openai_emb", "openai", "text-embedding-3-small").
    WithAPIKey(os.Getenv("OPENAI_API_KEY")).
    WithDimension(1536).
    Build(ctx)

// Vector DB
vectorDB, _ := knowledge.NewPgVector("pgvector").
    WithDSN("postgresql://localhost/vectors").
    WithDimension(1536).
    Build(ctx)

// Knowledge Base
kb, _ := knowledge.NewBase("docs").
    WithEmbedder("openai_emb").
    WithVectorDB("pgvector").
    AddSource(
        knowledge.NewMarkdownGlobSource("docs/**/*.md").Build(ctx),
    ).
    WithChunking("recursive_text_splitter", 512, 64).
    WithRetrieval(5, 0.2, 1200).
    Build(ctx)

// Add to project
proj.AddEmbedder(embedder).
    AddVectorDB(vectorDB).
    AddKnowledgeBase(kb)
```

### Example 4: Memory (Conversation State)

**Before (YAML):**
```yaml
# compozy.yaml
memories:
  - id: conversation
    type: token_based
    max_messages: 50
    persistence:
      type: redis
      ttl: 168h
    default_key_template: "user:{{.workflow.input.user_id}}"
```

```yaml
# agent.yaml
id: chat_assistant
memory:
  - id: conversation
    mode: read-write
    key: "user:{{.workflow.input.user_id}}"
```

**After (Go SDK):**
```go
import (
    "time"
    "github.com/compozy/compozy/v2/memory"
)

// Memory configuration
mem, _ := memory.New("conversation").
    WithType("token_based").
    WithMaxMessages(50).
    WithPersistence("redis", 168*time.Hour).
    WithDefaultKeyTemplate("user:{{.workflow.input.user_id}}").
    Build(ctx)

// Memory reference (in agent)
memRef, _ := memory.NewReference("conversation").
    WithMode("read-write").
    WithKey("user:{{.workflow.input.user_id}}").
    Build(ctx)

agent, _ := agent.New("chat_assistant").
    WithMemory(memRef).
    Build(ctx)
```

### Example 5: MCP Integration (External Tools)

**Before (YAML):**
```yaml
# compozy.yaml
mcps:
  - id: github
    transport: streamable-http
    url: "https://api.githubcopilot.com/mcp"
    headers:
      Authorization: "Bearer {{ .env.GITHUB_TOKEN }}"
```

**After (Go SDK):**
```go
import (
    "fmt"
    "os"
    "github.com/compozy/compozy/v2/mcp"
)

mcpCfg, _ := mcp.New("github").
    WithTransport("streamable-http").
    WithURL("https://api.githubcopilot.com/mcp").
    AddHeader("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GITHUB_TOKEN"))).
    Build(ctx)

proj.AddMCP(mcpCfg)
```

---

### Example 6: Runtime Configuration (JavaScript Tools)

**Before (YAML):**
```yaml
# compozy.yaml
runtime:
  type: bun
  entrypoint: "./tools.ts"
  permissions:
    - --allow-read
    - --allow-net=api.company.com
  tool_execution_timeout: 120s
  native_tools:
    root_dir: ../..
    call_agents:
      enabled: true
      max_concurrent: 3
      default_timeout: 120s
```

**After (Go SDK):**
```go
import (
    "time"
    "github.com/compozy/compozy/v2/runtime"
)

nativeTools, _ := runtime.NewNativeTools().
    WithRootDir("../..").
    EnableCallAgents(3, 120*time.Second).
    Build(ctx)

rt, _ := runtime.New("bun").
    WithEntrypoint("./tools.ts").
    AddPermission("--allow-read").
    AddPermission("--allow-net=api.company.com").
    WithToolExecutionTimeout(120 * time.Second).
    WithNativeTools(nativeTools).
    Build(ctx)

proj.WithRuntime(rt)
```

---

### Example 7: Custom Tools

**Before (YAML):**
```yaml
# compozy.yaml or workflow.yaml
tools:
  - id: file_reader
    description: Read and parse files
    timeout: 30s
    input:
      type: object
      properties:
        path:
          type: string
        format:
          type: string
          enum: [json, yaml, csv]
      required: [path]
    output:
      type: object
      properties:
        content:
          type: string
      required: [content]
```

**After (Go SDK):**
```go
import (
    "time"
    "github.com/compozy/compozy/v2/tool"
    "github.com/compozy/compozy/v2/schema"
)

toolCfg, _ := tool.New("file_reader").
    WithDescription("Read and parse files").
    WithTimeout(30 * time.Second).
    WithInputSchema(
        schema.New("file_input").
            ObjectType().
            AddProperty("path", schema.NewStringProperty()).
            AddProperty("format", schema.NewStringProperty().WithEnum("json", "yaml", "csv")).
            WithRequired("path").
            Build(ctx),
    ).
    WithOutputSchema(
        schema.New("file_output").
            ObjectType().
            AddProperty("content", schema.NewStringProperty()).
            WithRequired("content").
            Build(ctx),
    ).
    Build(ctx)

proj.AddTool(toolCfg)
```

---

### Example 8: Schema Validation

**Before (YAML):**
```yaml
# workflow.yaml
schemas:
  - id: user_input
    type: object
    properties:
      name:
        type: string
        minLength: 1
      email:
        type: string
        format: email
      age:
        type: integer
        minimum: 0
    required: [name, email]
```

**After (Go SDK):**
```go
import "github.com/compozy/compozy/v2/schema"

userSchema, _ := schema.New("user_input").
    ObjectType().
    AddProperty("name",
        schema.NewStringProperty().
            WithMinLength(1).
            Build(ctx),
    ).
    AddProperty("email",
        schema.NewStringProperty().
            WithFormat("email").
            Build(ctx),
    ).
    AddProperty("age",
        schema.NewIntegerProperty().
            WithMinimum(0).
            Build(ctx),
    ).
    WithRequired("name", "email").
    Build(ctx)

wf.AddSchema(userSchema)
```

---

### Example 9: Scheduled Workflows

**Before (YAML):**
```yaml
# workflow.yaml
id: daily_report
schedules:
  - cron: "0 0 9 * * 1-5"  # 9 AM weekdays
    timezone: "America/New_York"
    enabled: true
    jitter: "5m"
    overlap_policy: skip
    input:
      report_type: daily
```

**After (Go SDK):**
```go
import (
    "time"
    "github.com/compozy/compozy/v2/schedule"
    "github.com/compozy/compozy/v2/workflow"
)

sched, _ := schedule.New("0 0 9 * * 1-5").
    WithTimezone("America/New_York").
    Enabled(true).
    WithJitter(5 * time.Minute).
    WithOverlapPolicy("skip").
    WithInput(map[string]interface{}{
        "report_type": "daily",
    }).
    Build(ctx)

wf := workflow.New("daily_report").
    AddSchedule(sched).
    Build(ctx)
```

---

### Example 10: Signals (Inter-Workflow Communication)

**Before (YAML):**
```yaml
# sender-workflow.yaml
tasks:
  - id: send_signal
    type: signal_send
    signal_id: "data_ready"
    with:
      data: "{{ .tasks.process.output }}"

# receiver-workflow.yaml
tasks:
  - id: wait_signal
    type: signal_wait
    signal_id: "data_ready"
    timeout: 5m
    on_success:
      next: process_data
```

**After (Go SDK):**
```go
import (
    "time"
    "github.com/compozy/compozy/v2/task"
)

// Sender workflow (unified SignalBuilder)
sendTask, _ := task.NewSignal("send_signal").
    WithSignalID("data_ready").
    WithMode(task.SignalModeSend).
    WithData(map[string]interface{}{
        "data": "{{ .tasks.process.output }}",
    }).
    Build(ctx)

senderWF := workflow.New("sender").
    AddTask(sendTask).
    Build(ctx)

// Receiver workflow (unified SignalBuilder)
waitTask, _ := task.NewSignal("wait_signal").
    WithSignalID("data_ready").
    WithMode(task.SignalModeWait).
    WithTimeout(5 * time.Minute).
    OnSuccess("process_data").
    Build(ctx)

receiverWF := workflow.New("receiver").
    AddTask(waitTask).
    Build(ctx)
```

---


## Embedded Usage Pattern

The v2 SDK embeds Compozy directly in your Go application. Here's the complete pattern:

**Complete Embedded Example:**
```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/v2/compozy"
    "github.com/compozy/compozy/v2/project"
    // ... other builders
)

func main() {
    ctx := context.Background()

    // 1. Build project configuration
    proj, _ := project.New("my-app").
        AddModel(model.New("openai", "gpt-4").Build(ctx)).
        AddWorkflow(workflow.New("greet").Build(ctx)).
        Build(ctx)

    // 2. Embed Compozy with infrastructure config
    app, err := compozy.New(proj).
        WithServerPort(8080).
        WithDatabase("postgres://localhost/db").
        WithTemporal("localhost:7233", "default").
        WithRedis("redis://localhost:6379").
        Build(ctx)

    if err != nil {
        log.Fatalf("Init failed: %v", err)
    }

    // 3. Start embedded server (includes ALL HTTP endpoints)
    go func() {
        if err := app.Start(); err != nil {
            log.Printf("Server stopped: %v", err)
        }
    }()

    // 4. Execute workflows directly
    result, _ := app.ExecuteWorkflow(ctx, "greet", map[string]interface{}{
        "name": "Alice",
    })
    log.Printf("Result: %v", result.Output)

    // 5. Add custom endpoints
    router := app.Router()
    router.GET("/custom", myHandler)

    // Wait for shutdown
    app.Wait()
}
```

**Key Points:**
- ✅ No YAML files needed (pure Go)
- ✅ Server runs embedded (same process)
- ✅ All /api/v0/* endpoints automatic
- ✅ Direct workflow execution (no HTTP)
- ✅ Custom endpoints via app.Router()

---

## Common Patterns

### Pattern 1: Environment Variables

**YAML:**
```yaml
api_key: "{{ .env.OPENAI_API_KEY }}"
```

**Go SDK:**
```go
import "os"

WithAPIKey(os.Getenv("OPENAI_API_KEY"))
```

### Pattern 2: Template Expressions

**YAML:**
```yaml
with:
  data: "{{ .workflow.input.data }}"
```

**Go SDK:**
```go
WithInput(map[string]interface{}{
    "data": "{{ .workflow.input.data }}",
})
```

### Pattern 3: Conditional Logic

**YAML:**
```yaml
# Not easily supported
```

**Go SDK (Advantage!):**
```go
agent := agent.New("assistant")
if isPremium {
    agent.WithKnowledge(premiumKB).WithMemory(enhancedMemory)
}
```

### Pattern 4: Dynamic Workflows

**YAML:**
```yaml
# Not supported - static configuration
```

**Go SDK (Advantage!):**
```go
for _, customer := range customers {
    wf := BuildCustomerWorkflow(customer)
    client.Deploy(wf)
}
```

---

## Step-by-Step Migration

### Step 1: Set Up Go Project

```bash
mkdir my-compozy-app
cd my-compozy-app
go mod init myapp
go get github.com/compozy/compozy/v2@latest
```

### Step 2: Create main.go

```go
package main

import (
    "context"
    "github.com/compozy/compozy/v2/project"
    "github.com/compozy/compozy/v2/client"
)

func main() {
    ctx := context.Background()

    // Build project (converted from YAML)
    proj, _ := project.New("my-project").
        // ... add components
        Build(ctx)

    // Initialize embedded Compozy
    app, _ := compozy.New(proj).
        WithDatabase("postgres://localhost/db").
        WithTemporal("localhost:7233", "default").
        WithRedis("redis://localhost:6379").
        Build(ctx)

    // Start embedded server
    go app.Start()
}
```

### Step 3: Convert Components

Convert one component at a time:
1. Models → `model.New()`
2. Agents → `agent.New()`
3. Tasks → `task.NewBasic()`, etc.
4. Workflows → `workflow.New()`
5. Knowledge bases → `knowledge.NewBase()`
6. Memories → `memory.New()`

### Step 4: Initialize and Run

```go
// Initialize embedded Compozy
app, _ := compozy.New(proj).
    WithDatabase("postgres://localhost/db").
    WithTemporal("localhost:7233", "default").
    WithRedis("redis://localhost:6379").
    Build(ctx)

// Start server
go app.Start()

// Execute workflows
result, _ := app.ExecuteWorkflow(ctx, "workflow-id", input)
```

### Step 5: Test

```bash
go run main.go
```

### Step 5: Iterate

- Add validation
- Add error handling
- Add logging
- Add tests

---

## Benefits After Migration

### Type Safety
```go
// Compile-time error!
model.New("openai", "gpt-4").
    WithTemperature(5.0). // Error: must be 0.0-2.0
    Build(ctx)
```

### IDE Support
- Autocomplete for all methods
- Go to definition
- Find usages
- Refactoring

### Programmatic Control
```go
// Generate workflows dynamically
for _, customer := range customers {
    wf := GenerateWorkflow(customer)
    client.Deploy(wf)
}
```

### Unit Testing
```go
func TestWorkflowConfig(t *testing.T) {
    wf, _ := workflow.New("test").Build(t.Context())
    assert.Equal(t, "test", wf.ID)
}
```

---

## Troubleshooting

### Issue: Import errors
```
Error: cannot find package
```

**Solution:**
```bash
go get github.com/compozy/compozy/v2@latest
go mod tidy
```

### Issue: Validation errors
```
Error: workflow ID is required
```

**Solution:** All builders validate at Build(). Check error messages:
```go
wf, err := workflow.New("").Build(ctx)
if err != nil {
    fmt.Printf("Validation error: %v\n", err)
}
```

### Issue: Template expressions not working
```
Error: template syntax error
```

**Solution:** Template expressions work the same way in Go SDK:
```go
WithInput(map[string]interface{}{
    "data": "{{ .workflow.input.data }}", // Same as YAML
})
```

---

## Next Steps

1. ✅ Review migration guide
2. ✅ Try simple example
3. ✅ Convert one workflow
4. ✅ Test thoroughly
5. ✅ Migrate remaining workflows
6. ✅ Add tests
7. ✅ Deploy to production

---

**Status:** ✅ Complete migration guide
**Support:** GitHub Discussions for questions
