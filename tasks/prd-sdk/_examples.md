# Examples Plan (Referenced): Compozy GO SDK

## Conventions

- Keep examples minimal; link to canonical examples in `tasks/prd-sdk/05-examples.md`.
- Favor context-first code; every Build uses `ctx`.
- Provide run commands; avoid embedding long code here.

## Example Matrix

1. examples/sdk/01_simple_workflow
- Purpose: Minimal workflow with agent + action.
- Files: link users to `tasks/prd-sdk/05-examples.md` section “1. Simple Workflow”.
- Demonstrates: context-first Build(ctx), agent action + schema.
- Walkthrough: `go run sdk/examples/01_simple_workflow.go` (see PRD examples file).

2. examples/sdk/02_parallel_tasks
- Purpose: Parallel execution.
- Files: see “2. Parallel Task Execution” in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: task types, ParallelBuilder, outputs.
- Walkthrough: `go run sdk/examples/02_parallel_tasks.go`.

3. examples/sdk/04_knowledge_rag
- Purpose: Full RAG path.
- Files: see “3. Knowledge Base (RAG)” in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: embedder, vector DB, binding, retrieval params.
- Walkthrough: `go run sdk/examples/04_knowledge_rag.go`.

4. examples/sdk/05_memory_conversation
- Purpose: Memory configuration and references.
- Files: see memory example in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: flush strategies, TTL/privacy, agent reference with key template.
- Walkthrough: `go run sdk/examples/05_memory_conversation.go`.

5. examples/sdk/06_runtime_native_tools
- Purpose: Runtime + native tools.
- Files: see runtime/native tools example in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: Bun/Node/Deno, call_agents/call_workflows.
- Walkthrough: `go run sdk/examples/06_runtime_native_tools.go`.

6. examples/sdk/07_scheduled_workflow
- Purpose: Schedules.
- Files: see schedules example in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: cron, jitter, overlap policy.
- Walkthrough: `go run sdk/examples/07_scheduled_workflow.go`.

7. examples/sdk/08_mcp_integration
- Purpose: MCP integration with remote server.
- Files: see MCP example in `tasks/prd-sdk/05-examples.md`.
- Demonstrates: transport, headers, proto.
- Walkthrough: `go run sdk/examples/08_mcp_integration.go`.

8. examples/sdk/all_in_one
- Purpose: End-to-end feature mix.
- Files: see “Complete Project (all features)” and “Debugging and Error Handling”.
- Demonstrates: memory + knowledge + MCP + runtime + monitoring.
- Walkthrough: `go run sdk/examples/11_all_in_one.go`.

## Minimal YAML Shapes

```yaml
# Reference PRD examples for YAML counterparts
# See: tasks/prd-sdk/06-migration-guide.md
```

## Test & CI Coverage

- Integration tests for example flows live under SDK packages per `tasks/prd-sdk/07-testing-strategy.md`.
- Keep examples deterministic; external calls via env vars.

## Runbooks per Example

- Prereqs: documented per example in `tasks/prd-sdk/05-examples.md`.
- Commands: use the `go run sdk/examples/*.go` commands shown there.

## Acceptance Criteria

- Each example runs locally using the commands in PRD examples.
- Output matches expectations documented in `tasks/prd-sdk/05-examples.md`.
