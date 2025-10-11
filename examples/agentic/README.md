# Agent Orchestrator Example

This example pairs with the `cp__agent_orchestrate` documentation. It shows how to delegate work to multiple agents from a single tool call, combining sequential gathering with a parallel synthesis/review stage.

## Highlights

- Uses a structured plan with unique step IDs, required `status: pending`, and `result_key` bindings.
- Demonstrates sequential (research) and parallel (writer + reviewer) branches.
- Exposes planner-driven usage via API request so you can experiment with natural language prompts.

## Prerequisites

- A running Compozy backend (`make dev` from the repository root starts the services used by examples).
- An OpenAI API key exported as `OPENAI_API_KEY` (change the model/provider in `compozy.yaml` if you prefer another LLM).

```bash
export OPENAI_API_KEY=sk-...
```

## Run the workflow

1. Start the example runtime:

   ```bash
   cd examples/agentic
   ../../compozy dev
   ```

2. In another terminal, trigger the workflow with the provided [api.http](./api.http) request or curl:

   ```bash
   curl -X POST http://localhost:5001/api/v0/workflows/agentic/executions \
     -H 'Content-Type: application/json' \
     -d '{
       "input": { "topic": "Edge caching for e-commerce" }
     }'
   ```

3. Inspect the response to see:
   - The orchestrator output with `success`, `steps`, and collected `bindings`.
   - Each step's `exec_id` so you can drill into `/executions/agents/{exec_id}`.

## Prompt-driven orchestration

If you invoke `cp__agent_orchestrate` directly (via agent tool calling or the tools API), you can delegate planning to the builtin by sending a payload like:

```json
{
  "prompt": "Gather research on the topic, then summarize and critique it with our writer and reviewer agents.",
  "bindings": {
    "topic": "Edge caching for e-commerce"
  }
}
```

With the planner enabled, the builtin will compile a plan equivalent to the structured version under `workflows/orchestrate.yaml`.

## Files

- `compozy.yaml` – project manifest wiring the workflow and enabling the native tool.
- `workflows/orchestrate.yaml` – structured plan example executed by the builtin.
- `agents/*.yaml` – agent definitions called by the orchestrator steps.
- `api.http` – ready-to-run REST Client snippets.

Tweak the prompts, add additional agent steps, or raise the `runtime.native_tools.agent_orchestrator` limits to explore more complex orchestrations.
