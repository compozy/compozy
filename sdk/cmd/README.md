# Compozy SDK Examples

This directory contains runnable Go programs demonstrating core patterns in the Compozy SDK. Each example is organized in its own subdirectory under `sdk/cmd/` and follows the context-first approach required across the project: loggers and configuration managers are attached to `context.Context` before any builders are invoked, and all resources are constructed by calling `Build(ctx)`.

## 01. Simple Workflow

Located in `sdk/cmd/01_simple_workflow/`. This is the "hello world" example that shows how to:

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
go run ./sdk/cmd/01_simple_workflow
```

The program logs each build step, prints a summary of the resulting project configuration, and warns when required environment variables are missing.

## 02. Parallel Tasks

Located in `sdk/cmd/02_parallel_tasks/`. This example demonstrates how to orchestrate concurrent analysis pipelines while keeping task outputs aligned for downstream consumers. It highlights how to:

- Build three specialized agents (sentiment, entity extraction, summarization)
- Configure individual basic tasks that share workflow input via templates
- Execute the tasks concurrently with `ParallelBuilder` using `WithWaitAll(true)` to guarantee consistent aggregation
- Merge the branch outputs with `AggregateBuilder` using the `merge` strategy to produce a single payload
- Surface aggregated results as workflow outputs so other systems can consume a unified analysis artifact

### Run the Example

```bash
go run ./sdk/cmd/02_parallel_tasks
```

Use the same `OPENAI_API_KEY` export from the simple workflow example; the builders log a warning when credentials are missing.

When the program finishes it prints a summary showing how many tasks run in parallel and how aggregation reduces fan-out for calling services.

### What's Next

Upcoming examples in this directory will introduce knowledge bases, long-term memory, MCP integrations, and more advanced orchestration scenarios. Every example continues to reinforce context-first patterns and robust error handling with `BuildError`.

## 03. Knowledge RAG

Located in `sdk/cmd/03_knowledge_rag/`. This example assembles a complete retrieval augmented generation pipeline with all five knowledge builders. It walks through configuring an OpenAI embedder, provisioning a pgvector-backed collection, wiring local file and directory sources alongside remote documentation URLs, and binding the resulting knowledge base to an agent inside a runnable workflow.

### Prerequisites

```bash
export OPENAI_API_KEY="sk-..."
export PGVECTOR_DSN="postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable"
```

Ensure the referenced PostgreSQL instance has the `pgvector` extension installed and reachable via the DSN above.

### Run the Example

```bash
go run ./sdk/cmd/03_knowledge_rag
```

The program prints a summary confirming that the embedder, vector database, knowledge base, and agent binding were all constructed successfully, highlighting chunking, retrieval, and binding parameters used during ingestion.

## 04. Memory Conversation

Located in `sdk/cmd/04_memory_conversation/`. This example demonstrates the full memory subsystem, wiring a support agent to durable conversation state. It highlights how to:

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
go run ./sdk/cmd/04_memory_conversation
```

The program emits a summary covering the memory resource, privacy scope, expiration window, and distributed locking state so you can validate that all advanced features are enabled.

## 05. MCP Integration

Located in `sdk/cmd/05_mcp_integration/`. This example configures three MCP transports side-by-side to illustrate how remote HTTP (SSE), local stdio, and containerized stdio servers coexist inside an SDK project. The example demonstrates how to:

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
go run ./sdk/cmd/05_mcp_integration
```

The program prints a summary of each MCP configuration, including transport type, headers, environment variables, and startup timeouts so you can confirm every integration path is wired correctly.

## 06. Runtime + Native Tools

Located in `sdk/cmd/06_runtime_native_tools/`. This example demonstrates how to configure Bun runtimes alongside native tool capabilities while contrasting the three runtime profiles (Bun sandbox, Node compatibility, and inherited global settings). It highlights how to:

- Enable `cp__call_agents` and `cp__call_workflows` natively inside the Bun sandbox
- Tune Bun permissions and memory ceilings to satisfy security constraints
- Document alternative runtime types (Node and inherited global runtime) for compatibility scenarios
- Attach the resulting runtime configuration to a project so downstream services pick it up automatically

### Run the Example

```bash
go run ./sdk/cmd/06_runtime_native_tools
```

The example logs a structured summary for each runtime profile so operators can audit permissions, native tool availability, and memory limits before deploying real TypeScript or JavaScript entrypoints.

## 07. Scheduled Workflow

Located in `sdk/cmd/07_scheduled_workflow/`. This example shows how to attach multiple cron-based schedules to a single workflow. It covers:

- Building a workflow once and referencing it from daily and weekly schedules
- Using the schedule builder to declare cron expressions, default input payloads, and retry policies
- Explaining cron syntax inline so operators understand the cadence at a glance
- Registering schedules on the project builder so they ship with the compiled configuration

### Run the Example

```bash
go run ./sdk/cmd/07_scheduled_workflow
```

The program logs each schedule with its cron expression, retry configuration, and input keys so you can confirm the automation profile the project will register.

## 08. Signal Communication

Located in `sdk/cmd/08_signal_communication/`. This example demonstrates how two workflows coordinate through the unified signal builder. It highlights how to:

- Build a processing workflow that sends a readiness signal with structured payload data
- Configure a downstream workflow that waits on the same signal with an explicit timeout window
- Share agents and models while keeping context-first patterns for logging and configuration
- Summarize signal metadata so operators can confirm payload fields, wait targets, and timeout durations

### Run the Example

```bash
go run ./sdk/cmd/08_signal_communication
```

The program prints context-aware logs that illustrate when to use `Send()` versus `Wait()`, how payload keys propagate to receivers, and how timeouts guard against stalled upstream workflows.

## 10. Complete Project

Located in `sdk/cmd/10_complete_project/`. This is the kitchen-sink reference: it assembles every SDK builder into a single runnable project, wires live integrations, and boots an embedded Compozy instance end-to-end. You can treat it as the canonical blueprint when you need to remember how two or more subsystems snap together.

### Prerequisites

- PostgreSQL with the `pgvector` extension (`PGVECTOR_DSN`)
- Redis (`REDIS_URL`)
- Temporal server reachable at `localhost:7233`
- API keys: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`
- Optional: ensure Bun runtime is installed if you plan to execute the generated tool code

### Run the Example

```bash
go run ./sdk/cmd/10_complete_project
```

The program emits a structured summary detailing models, knowledge bases, schedules, monitoring endpoints (`http://127.0.0.1:18080/metrics`), and workflow outputs.

### Feature Guide

- Demonstrates all 30 SDK builders (models, knowledge, memory, MCP, runtime, tools, tasks, workflows, schedules, comps)
- Configures Prometheus metrics and Jaeger-compatible tracing fields via `config.FromContext(ctx)`
- Starts an embedded server, executes a workflow, and gracefully shuts down using the lifecycle APIs
- Surfaces helper functions for grouping related builder responsibilities into small, testable units

### Debugging Guide

- Watch the console for aggregated builder logs (each build emits `"building ..."` debug entries)
- Query `/metrics` to confirm Prometheus export, and attach Jaeger to the configured endpoint for spans
- Use the returned workflow output to validate template expressions (e.g., aggregated summaries, signal payloads)

### Common Issues

- Missing environment variables: the builder logs warnings and the execution will return validation errors
- Temporal, Redis, or PostgreSQL offline: the embedded server will fail to start—check Docker/Compose status
- Port conflicts on `18080`: adjust `WithServerPort` in the example before running

## 11. Debugging Toolkit

Located in `sdk/cmd/11_debugging/`. This example focuses on troubleshooting techniques rather than orchestration. It shows how to accumulate build errors, inspect configs, validate manually, measure builder latency, and set up contextual logging.

### Run the Example

```bash
go run ./sdk/cmd/11_debugging
```

### What You’ll See

- Aggregated `BuildError` output with every individual validation failure listed
- `AsMap()` inspection of a constructed agent for quick debugging or serialization
- Manual project validation flow demonstrating when to call `Validate(ctx)`
- Micro benchmark for builder latency with a thin wait task
- `logger.FromContext(ctx)` usage to emit debug lines without passing loggers around

### Common Issues

- Running outside the repository root will prevent the README from resolving during knowledge inspection; stay in the repo root
- If the example panics, confirm Go 1.25.2+ is active—older toolchains may not support the generics utilities used in the context setup
