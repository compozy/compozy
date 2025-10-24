# Agentic Research Tasks Example

This example demonstrates how to autoload agents and tasks, then execute them directly through the Tasks API.  
It highlights the new `cp__call_agents` builtin by orchestrating three research agents in parallel.

## Highlights

- Agents and tasks are auto-discovered (`agents/*.yaml`, `tasks/*.yaml`) – no workflow manifest required.
- `topic-research` shows a classic single-agent call.
- `topic-research-multi-agent` uses `cp__call_agents` to run academic, industry, and web researchers in parallel and aggregate the results.
- REST client snippets in [api.http](./api.http) trigger synchronous and asynchronous task executions.

## Prerequisites

- A running Compozy backend (`make dev` from the repository root starts the services).
- A Groq API key exported as `GROQ_API_KEY` (the manifest uses `openai/gpt-oss-120b` via Groq by default).  
  Adjust `compozy.yaml` if you prefer a different provider/model.

```bash
export GROQ_API_KEY=grq_...
```

## Run the tasks

1. Start the example runtime:

   ```bash
   cd examples/agentic
   ../../compozy dev
   ```

2. Execute the sample requests from [api.http](./api.http) or curl:

   ```bash
   # Single-agent task (synchronous)
   curl -X POST http://localhost:5001/api/v0/tasks/topic-research/executions/sync \
     -H 'Content-Type: application/json' \
     -d '{ "with": { "topic": "Edge caching for e-commerce" } }'
   
   # Multi-agent task (asynchronous)
   curl -X POST http://localhost:5001/api/v0/tasks/topic-research-multi-agent/executions \
     -H 'Content-Type: application/json' \
     -d '{ "with": { "topic": "Edge caching for e-commerce" } }'
   ```

3. For async execution, poll `GET /executions/tasks/{exec_id}` until status is `completed`.

## Auto-loaded resources

- `agents/research-*.yaml` – three agents with different research lenses (web, academic, industry).
- `tasks/topic-research.yaml` – single-agent task mirroring the original example behaviour.
- `tasks/topic-research-multi-agent.yaml` – calls all three agents via `cp__call_agents` and stitches the responses together.
- `compozy.yaml` – project manifest with autoload settings and runtime defaults.
- `api.http` – REST Client snippets for both sync and async task execution.

Feel free to tailor the agent instructions, add new variations, or adjust the task outputs to fit your experimentation needs.
