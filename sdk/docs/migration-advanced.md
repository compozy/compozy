# Advanced Migration Patterns

Extend the quick-start guidance in [Migration Basics](./migration-basics.md) with hybrid deployments, advanced resources, and embedded runtime patterns. Every example below follows the context-first rule: derive a request-scoped `ctx` that already carries configuration and logger instances via `config.ContextWithManager` and `logger.ContextWithLogger` before calling any builder `Build(ctx)`.

## Migration Strategies

1. Keep YAML only when delivery cadence is low and operations teams already own the templates.
2. Adopt a hybrid SDK + YAML model to layer typed builders on top of existing assets while AutoLoad discovers legacy resources.
3. Move to full SDK usage when you need programmatic generation, static analysis, or test suites around configuration.

### Decision Tree

| If you need…                                                    | Recommended path | Why it fits                                                                   |
| --------------------------------------------------------------- | ---------------- | ----------------------------------------------------------------------------- |
| Zero change risk, existing YAML ownership                       | Keep YAML        | Continue shipping without refactors.                                          |
| Incremental rollout of SDK builders, reuse of YAML tools/models | Hybrid           | AutoLoad registers YAML after SDK resources, enabling side-by-side execution. |
| Rich CI validation, dynamic composition, shared Go libraries    | Full SDK         | Everything lives in Go, enabling build-time checks and reuse.                 |

## Hybrid Projects with AutoLoad

Hybrid projects register SDK-defined resources first, then opt in to YAML discovery. AutoLoad accepts include and exclude globs so teams can migrate folder by folder without breaking references.

```go
package hybrid

import (
    "context"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/agent"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/workflow"
)

func configureHybrid(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    orchestrator, err := agent.New("orchestrator").
        WithInstructions("Coordinate YAML tool invocations").
        Build(ctx)
    if err != nil {
        return nil, err
    }

    orchestrate, err := workflow.New("yaml-orchestrator").
        AddAgent(orchestrator).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("customer-success").
        WithDescription("Hybrid SDK + YAML example").
        AddWorkflow(orchestrate).
        WithAutoLoad(true, []string{"yaml/**/*.yaml"}, []string{"yaml/deprecated/*.yaml"}).
        Build(ctx)
}
```

**Hybrid notes**

- SDK registrations run first; AutoLoad then walks include globs and attaches YAML resources.
- `$ref` works across SDK and YAML IDs as long as they stay unique.
- Disable AutoLoad (`WithAutoLoad(false, nil, nil)`) once YAML assets are fully migrated.

## Advanced Feature Examples (Examples 3–10)

All scenarios below map directly to the numbered examples in `tasks/prd-sdk/06-migration-guide.md` and demonstrate full imports plus context usage.

### Example 3: Knowledge Base (RAG)

**Before (YAML)**

```yaml
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

**After (Go SDK)**

```go
package knowledgebase

import (
    "context"
    "os"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/knowledge"
    "github.com/compozy/compozy/sdk/project"
)

func configureKnowledge(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    embedder, err := knowledge.NewEmbedder("openai_emb", "openai", "text-embedding-3-small").
        WithAPIKey(os.Getenv("OPENAI_API_KEY")).
        WithDimension(1536).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    vectorDB, err := knowledge.NewPgVector("pgvector").
        WithDSN("postgresql://localhost/vectors").
        WithDimension(1536).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    kb, err := knowledge.NewBase("docs").
        WithEmbedder("openai_emb").
        WithVectorDB("pgvector").
        AddSource(knowledge.NewMarkdownGlobSource("docs/**/*.md").Build(ctx)).
        WithChunking("recursive_text_splitter", 512, 64).
        WithRetrieval(5, 0.2, 1200).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("docs-project").
        AddEmbedder(embedder).
        AddVectorDB(vectorDB).
        AddKnowledgeBase(kb).
        Build(ctx)
}
```

### Example 4: Memory Configuration

**Before (YAML)**

```yaml
memories:
  - id: conversation
    type: token_based
    max_messages: 50
    persistence:
      type: redis
      ttl: 168h
    default_key_template: "user:{{.workflow.input.user_id}}"

agents:
  - id: chat_assistant
    memory:
      - id: conversation
        mode: read-write
        key: "user:{{.workflow.input.user_id}}"
```

**After (Go SDK)**

```go
package memoryconfig

import (
    "context"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/agent"
    "github.com/compozy/compozy/sdk/memory"
    "github.com/compozy/compozy/sdk/project"
)

func configureMemory(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    memConfig, err := memory.New("conversation").
        WithType("token_based").
        WithMaxMessages(50).
        WithPersistence("redis", 168*time.Hour).
        WithDefaultKeyTemplate("user:{{.workflow.input.user_id}}").
        Build(ctx)
    if err != nil {
        return nil, err
    }

    memRef, err := memory.NewReference("conversation").
        WithMode("read-write").
        WithKey("user:{{.workflow.input.user_id}}").
        Build(ctx)
    if err != nil {
        return nil, err
    }

    assistant, err := agent.New("chat_assistant").
        WithInstructions("Conversational assistant with shared memory").
        WithMemory(memRef).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("memory-project").
        AddMemory(memConfig).
        AddAgent(assistant).
        Build(ctx)
}
```

### Example 5: MCP Integration

**Before (YAML)**

```yaml
mcps:
  - id: github
    transport: streamable-http
    url: "https://api.githubcopilot.com/mcp"
    headers:
      Authorization: "Bearer {{ .env.GITHUB_TOKEN }}"
```

**After (Go SDK)**

```go
package mcpconfig

import (
    "context"
    "fmt"
    "os"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/mcp"
    "github.com/compozy/compozy/sdk/project"
)

func configureMCP(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    mcpServer, err := mcp.New("github").
        WithTransport("streamable-http").
        WithURL("https://api.githubcopilot.com/mcp").
        AddHeader("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GITHUB_TOKEN"))).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("mcp-project").
        AddMCP(mcpServer).
        Build(ctx)
}
```

### Example 6: Runtime + Native Tools

**Before (YAML)**

```yaml
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

**After (Go SDK)**

```go
package runtimeconfig

import (
    "context"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/runtime"
)

func configureRuntime(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    nativeTools, err := runtime.NewNativeTools().
        WithRootDir("../..").
        EnableCallAgents(3, 120*time.Second).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    rt, err := runtime.New("bun").
        WithEntrypoint("./tools.ts").
        AddPermission("--allow-read").
        AddPermission("--allow-net=api.company.com").
        WithToolExecutionTimeout(120 * time.Second).
        WithNativeTools(nativeTools).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("runtime-project").
        WithRuntime(rt).
        Build(ctx)
}
```

### Example 7: Custom Tool Registration

**Before (YAML)**

```yaml
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

**After (Go SDK)**

```go
package toolsconfig

import (
    "context"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/schema"
    "github.com/compozy/compozy/sdk/tool"
)

func configureTools(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    toolCfg, err := tool.New("file_reader").
        WithDescription("Read and parse files").
        WithTimeout(30 * time.Second).
        WithInputSchema(
            schema.New("file_reader_input").
                ObjectType().
                AddProperty("path", schema.NewStringProperty().Build(ctx)).
                AddProperty("format", schema.NewStringProperty().WithEnum("json", "yaml", "csv").Build(ctx)).
                WithRequired("path").
                Build(ctx),
        ).
        WithOutputSchema(
            schema.New("file_reader_output").
                ObjectType().
                AddProperty("content", schema.NewStringProperty().Build(ctx)).
                WithRequired("content").
                Build(ctx),
        ).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("tools-project").
        AddTool(toolCfg).
        Build(ctx)
}
```

### Example 8: Schema Validation

**Before (YAML)**

```yaml
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

**After (Go SDK)**

```go
package schemaconfig

import (
    "context"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/schema"
)

func configureSchema(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    userSchema, err := schema.New("user_input").
        ObjectType().
        AddProperty("name", schema.NewStringProperty().WithMinLength(1).Build(ctx)).
        AddProperty("email", schema.NewStringProperty().WithFormat("email").Build(ctx)).
        AddProperty("age", schema.NewIntegerProperty().WithMinimum(0).Build(ctx)).
        WithRequired("name", "email").
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("schema-project").
        AddSchema(userSchema).
        Build(ctx)
}
```

### Example 9: Scheduled Workflows

**Before (YAML)**

```yaml
id: daily_report
schedules:
  - cron: "0 0 9 * * 1-5"
    timezone: "America/New_York"
    enabled: true
    jitter: "5m"
    overlap_policy: skip
    input:
      report_type: daily
```

**After (Go SDK)**

```go
package scheduleconfig

import (
    "context"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/schedule"
    "github.com/compozy/compozy/sdk/workflow"
)

func configureSchedule(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    sched, err := schedule.New("0 0 9 * * 1-5").
        WithTimezone("America/New_York").
        Enabled(true).
        WithJitter(5 * time.Minute).
        WithOverlapPolicy("skip").
        WithInput(map[string]any{"report_type": "daily"}).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    wf, err := workflow.New("daily_report").
        AddSchedule(sched).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("schedule-project").
        AddWorkflow(wf).
        Build(ctx)
}
```

### Example 10: Signals (Unified Builder)

**Before (YAML)**

```yaml
# sender
- id: send_signal
  type: signal_send
  signal_id: data_ready
  with:
    data: "{{ .tasks.process.output }}"

# receiver
- id: wait_signal
  type: signal_wait
  signal_id: data_ready
  timeout: 5m
  on_success:
    next: process_data
```

**After (Go SDK)**

```go
package signalconfig

import (
    "context"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/task"
    "github.com/compozy/compozy/sdk/workflow"
)

func configureSignals(parentCtx context.Context, mgr *config.Manager, log logger.Logger) (*project.Config, error) {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), log)

    sendSignal, err := task.NewSignal("send_signal").
        WithSignalID("data_ready").
        WithMode(task.SignalModeSend).
        WithData(map[string]any{"data": "{{ .tasks.process.output }}"}).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    sender, err := workflow.New("sender").
        AddTask(sendSignal).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    waitSignal, err := task.NewSignal("wait_signal").
        WithSignalID("data_ready").
        WithMode(task.SignalModeWait).
        WithTimeout(5 * time.Minute).
        OnSuccess("process_data").
        Build(ctx)
    if err != nil {
        return nil, err
    }

    receiver, err := workflow.New("receiver").
        AddTask(waitSignal).
        Build(ctx)
    if err != nil {
        return nil, err
    }

    return project.New("signals-project").
        AddWorkflow(sender).
        AddWorkflow(receiver).
        Build(ctx)
}
```

## Embedded Usage Pattern

Embed Compozy alongside your application logic to run the control plane in-process while still following context-first patterns.

```go
package embedded

import (
    "context"
    "log"
    "net/http"

    "github.com/gin-gonic/gin"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/sdk/compozy"
    "github.com/compozy/compozy/sdk/project"
    "github.com/compozy/compozy/sdk/workflow"
)

func runEmbedded(parentCtx context.Context, mgr *config.Manager, logProvider logger.Logger) error {
    ctx := logger.ContextWithLogger(config.ContextWithManager(parentCtx, mgr), logProvider)

    wf, err := workflow.New("greet").
        Build(ctx)
    if err != nil {
        return err
    }

    proj, err := project.New("embedded-app").
        AddWorkflow(wf).
        Build(ctx)
    if err != nil {
        return err
    }

    app, err := compozy.New(proj).
        WithServerPort(8080).
        WithDatabase("postgres://localhost/db").
        WithTemporal("localhost:7233", "default").
        WithRedis("redis://localhost:6379").
        Build(ctx)
    if err != nil {
        return err
    }

    go func() {
        if startErr := app.Start(); startErr != nil {
            logger.FromContext(ctx).Error("server stopped", "error", startErr)
        }
    }()

    result, err := app.ExecuteWorkflow(ctx, "greet", map[string]any{"name": "Ada"})
    if err != nil {
        return err
    }
    log.Printf("workflow output: %v", result.Output)

    routing := app.Router()
    routing.GET("/healthz", func(c *gin.Context) {
        ctx := c.Request.Context()
        logger.FromContext(ctx).Info("health check pong")
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })

    return app.Wait()
}
```

**Lifecycle checklist**

- Attach config manager and logger to every derived context before calling SDK builders or lifecycle methods.
- Start the embedded server in its own goroutine, capture shutdown errors, and block on `Wait()` for graceful termination.
- Use `app.Router()` to extend the HTTP surface while reusing the shared context.

## Applying This Guide

- Use the decision table to align migration scope with delivery risk.
- Pair Hybrid AutoLoad with targeted include globs to stage rollouts per directory.
- Reuse the advanced snippets as templates; each shows how to pull loggers and configuration from context before building resources.
- Link back to the basics guide for Example 1–2 patterns whenever teammates need a refresher on initialization or error handling fundamentals.
