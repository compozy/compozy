# Compozy SDK Examples

This directory contains runnable Go programs demonstrating core patterns in the Compozy SDK. Each example follows the context-first approach required across the project: loggers and configuration managers are attached to `context.Context` before any builders are invoked, and all resources are constructed by calling `Build(ctx)`.

## 01. Simple Workflow

The file `01_simple_workflow.go` is the "hello world" example. It shows how to:

- Bootstrap a context with logger and configuration manager
- Configure an OpenAI model using the model builder
- Define an agent action with a JSON Schema output
- Register an agent and basic task
- Assemble a workflow and project configuration
- Handle aggregated validation failures via `BuildError`

### Prerequisites

Set your OpenAI API key before running the example:

```bash
export OPENAI_API_KEY="sk-..."
```

### Run the Example

Execute the program from the repository root:

```bash
go run ./sdk/examples/01_simple_workflow.go
```

The program logs each build step, prints a summary of the resulting project configuration, and warns when required environment variables are missing.

## 02. Parallel Tasks

The file `02_parallel_tasks.go` demonstrates how to orchestrate concurrent analysis pipelines while keeping task outputs aligned for downstream consumers. It highlights how to:

- Build three specialized agents (sentiment, entity extraction, summarization)
- Configure individual basic tasks that share workflow input via templates
- Execute the tasks concurrently with `ParallelBuilder` using `WithWaitAll(true)` to guarantee consistent aggregation
- Merge the branch outputs with `AggregateBuilder` using the `merge` strategy to produce a single payload
- Surface aggregated results as workflow outputs so other systems can consume a unified analysis artifact

### Run the Example

```bash
go run ./sdk/examples/02_parallel_tasks.go
```

Use the same `OPENAI_API_KEY` export from the simple workflow example; the builders log a warning when credentials are missing.

When the program finishes it prints a summary showing how many tasks run in parallel and how aggregation reduces fan-out for calling services.

### What's Next

Upcoming examples in this directory will introduce knowledge bases, long-term memory, MCP integrations, and more advanced orchestration scenarios. Every example continues to reinforce context-first patterns and robust error handling with `BuildError`.

## 03. Knowledge RAG

The file `03_knowledge_rag.go` assembles a complete retrieval augmented generation pipeline with all five knowledge builders. It walks through configuring an OpenAI embedder, provisioning a pgvector-backed collection, wiring local file and directory sources alongside remote documentation URLs, and binding the resulting knowledge base to an agent inside a runnable workflow.

### Prerequisites

```bash
export OPENAI_API_KEY="sk-..."
export PGVECTOR_DSN="postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable"
```

Ensure the referenced PostgreSQL instance has the `pgvector` extension installed and reachable via the DSN above.

### Run the Example

```bash
go run ./sdk/examples/03_knowledge_rag.go
```

The program prints a summary confirming that the embedder, vector database, knowledge base, and agent binding were all constructed successfully, highlighting chunking, retrieval, and binding parameters used during ingestion.

## 04. Memory Conversation

The file `04_memory_conversation.go` demonstrates the full memory subsystem, wiring a support agent to durable conversation state. It highlights how to:

- Configure token-aware memory with summarization flush, user-scoped privacy, 24-hour expiration, Redis persistence, and distributed locking
- Build a dynamic memory reference so each conversation and user pair receives an isolated namespace
- Attach the memory reference to an agent and expose the memory resource inside a runnable workflow and project
- Summarize the resulting configuration so operators can verify persistence and locking policies before deployment

### Prerequisites

```bash
export OPENAI_API_KEY="sk-..."
export REDIS_URL="redis://localhost:6379/0"
```

Ensure the Redis instance referenced by `REDIS_URL` is reachable; the example prints the URL so you can confirm the connection target that the runtime will use.

### Run the Example

```bash
go run ./sdk/examples/04_memory_conversation.go
```

The program emits a summary covering the memory resource, privacy scope, expiration window, and distributed locking state so you can validate that all advanced features are enabled.

## 05. MCP Integration

The file `05_mcp_integration.go` configures three MCP transports side-by-side to illustrate how remote HTTP (SSE), local stdio, and containerized stdio servers coexist inside an SDK project. The example demonstrates how to:

- Bootstrap MCP definitions with headers, auth tokens, protocol selection, and session caps for a remote GitHub server
- Launch a filesystem MCP over stdio with environment variables and tight startup timeouts
- Run a PostgreSQL MCP inside Docker while still exposing stdio transport to the SDK
- Register all MCP servers on an agent, attach the agent to a workflow, and aggregate the configuration into a runnable project

### Prerequisites

```bash
export GITHUB_TOKEN="ghp-..."
export MCP_FS_ROOT="/path/to/workspace"
export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable"
```

### Run the Example

```bash
go run ./sdk/examples/05_mcp_integration.go
```

The program prints a summary of each MCP configuration, including transport type, headers, environment variables, and startup timeouts so you can confirm every integration path is wired correctly.

## 06. Runtime + Native Tools

The file `06_runtime_native_tools.go` demonstrates how to configure Bun runtimes alongside native tool capabilities while contrasting the three runtime profiles (Bun sandbox, Node compatibility, and inherited global settings). It highlights how to:

- Enable `cp__call_agents` and `cp__call_workflows` natively inside the Bun sandbox
- Tune Bun permissions and memory ceilings to satisfy security constraints
- Document alternative runtime types (Node and inherited global runtime) for compatibility scenarios
- Attach the resulting runtime configuration to a project so downstream services pick it up automatically

### Run the Example

```bash
go run ./sdk/examples/06_runtime_native_tools.go
```

The example logs a structured summary for each runtime profile so operators can audit permissions, native tool availability, and memory limits before deploying real TypeScript or JavaScript entrypoints.

## 07. Scheduled Workflow

The file `07_scheduled_workflow.go` shows how to attach multiple cron-based schedules to a single workflow. It covers:

- Building a workflow once and referencing it from daily and weekly schedules
- Using the schedule builder to declare cron expressions, default input payloads, and retry policies
- Explaining cron syntax inline so operators understand the cadence at a glance
- Registering schedules on the project builder so they ship with the compiled configuration

### Run the Example

```bash
go run ./sdk/examples/07_scheduled_workflow.go
```

The program logs each schedule with its cron expression, retry configuration, and input keys so you can confirm the automation profile the project will register.

## 08. Signal Communication

The file `08_signal_communication.go` demonstrates how two workflows coordinate through the unified signal builder. It highlights how to:

- Build a processing workflow that sends a readiness signal with structured payload data
- Configure a downstream workflow that waits on the same signal with an explicit timeout window
- Share agents and models while keeping context-first patterns for logging and configuration
- Summarize signal metadata so operators can confirm payload fields, wait targets, and timeout durations

### Run the Example

```bash
go run ./sdk/examples/08_signal_communication.go
```

The program prints context-aware logs that illustrate when to use `Send()` versus `Wait()`, how payload keys propagate to receivers, and how timeouts guard against stalled upstream workflows.
