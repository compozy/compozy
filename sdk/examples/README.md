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
