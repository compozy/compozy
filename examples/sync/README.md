# Sync Execution Example

This example demonstrates the new synchronous execution surfaces introduced in the PRD for `/sync` endpoints. It ships a single workflow, single agent, and single reusable task so you can exercise the API without extra setup.

## Highlights

- `POST /api/v0/workflows/{workflow_id}/executions/sync` returns the final workflow output in one request.
- `POST /api/v0/agents/{agent_id}/executions` triggers the same agent directly with a prompt payload.
- `POST /api/v0/tasks/{task_id}/executions` executes the task (which wraps the agent) without running the whole workflow.
- `api.http` contains ready-to-run requests for all three synchronous endpoints.

## Project layout

- `compozy.yaml` wires the workflow, agent, and task resources.
- `workflows/sync.yaml` defines the single-step workflow that forwards the caller message to the agent.
- `agents/sync-assistant.yaml` keeps the agent instructions and JSON contract in one place.
- `tasks/sync-respond.yaml` provides a reusable task that is invoked by both the workflow and the direct `/tasks` endpoint.
- `api.http` includes ready-to-run HTTP snippets for the workflow, agent, and task synchronous routes.

## Running locally

1. Export an LLM key supported by the selected provider (e.g. `export GROQ_API_KEY=...`).
2. Start Compozy from this example directory:
   ```bash
   cd examples/sync
   ../../compozy dev
   ```
3. Use the requests in `api.http` (VS Code REST client, Insomnia, or `curl`) to try each synchronous endpoint.

Each request returns JSON with the acknowledgement coming from the `sync-assistant` agent so you can validate behaviour quickly.
