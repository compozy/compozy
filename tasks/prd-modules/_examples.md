# Examples Plan (Referenced): Compozy v2 Go SDK

## Conventions

- Keep examples minimal; link to canonical examples in `tasks/prd-modules/05-examples.md`.
- Favor context-first code; every Build uses `ctx`.
- Provide run commands; avoid embedding long code here.

## Example Matrix

1. examples/v2/01_simple_workflow
- Purpose: Minimal workflow with agent + action.
- Files: link users to `tasks/prd-modules/05-examples.md` section “1. Simple Workflow”.
- Demonstrates: context-first Build(ctx), agent action + schema.
- Walkthrough: `go run v2/examples/01_simple_workflow.go` (see PRD examples file).

2. examples/v2/02_parallel_tasks
- Purpose: Parallel execution.
- Files: see “2. Parallel Task Execution” in `tasks/prd-modules/05-examples.md`.
- Demonstrates: task types, ParallelBuilder, outputs.
- Walkthrough: `go run v2/examples/02_parallel_tasks.go`.

3. examples/v2/04_knowledge_rag
- Purpose: Full RAG path.
- Files: see “3. Knowledge Base (RAG)” in `tasks/prd-modules/05-examples.md`.
- Demonstrates: embedder, vector DB, binding, retrieval params.
- Walkthrough: `go run v2/examples/04_knowledge_rag.go`.

4. examples/v2/05_memory_conversation
- Purpose: Memory configuration and references.
- Files: see memory example in `tasks/prd-modules/05-examples.md`.
- Demonstrates: flush strategies, TTL/privacy, agent reference with key template.
- Walkthrough: `go run v2/examples/05_memory_conversation.go`.

5. examples/v2/06_runtime_native_tools
- Purpose: Runtime + native tools.
- Files: see runtime/native tools example in `tasks/prd-modules/05-examples.md`.
- Demonstrates: Bun/Node/Deno, call_agents/call_workflows.
- Walkthrough: `go run v2/examples/06_runtime_native_tools.go`.

6. examples/v2/07_scheduled_workflow
- Purpose: Schedules.
- Files: see schedules example in `tasks/prd-modules/05-examples.md`.
- Demonstrates: cron, jitter, overlap policy.
- Walkthrough: `go run v2/examples/07_scheduled_workflow.go`.

7. examples/v2/08_mcp_integration
- Purpose: MCP integration with remote server.
- Files: see MCP example in `tasks/prd-modules/05-examples.md`.
- Demonstrates: transport, headers, proto.
- Walkthrough: `go run v2/examples/08_mcp_integration.go`.

8. examples/v2/all_in_one
- Purpose: End-to-end feature mix.
- Files: see “Complete Project (all features)” and “Debugging and Error Handling”.
- Demonstrates: memory + knowledge + MCP + runtime + monitoring.
- Walkthrough: `go run v2/examples/11_all_in_one.go`.

## Minimal YAML Shapes

```yaml
# Reference PRD examples for YAML counterparts
# See: tasks/prd-modules/06-migration-guide.md
```

## Test & CI Coverage

- Integration tests for example flows live under SDK packages per `tasks/prd-modules/07-testing-strategy.md`.
- Keep examples deterministic; external calls via env vars.

## Runbooks per Example

- Prereqs: documented per example in `tasks/prd-modules/05-examples.md`.
- Commands: use the `go run v2/examples/*.go` commands shown there.

## Acceptance Criteria

- Each example runs locally using the commands in PRD examples.
- Output matches expectations documented in `tasks/prd-modules/05-examples.md`.
