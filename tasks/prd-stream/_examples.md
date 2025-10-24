# Examples Plan — Executions Streaming (/stream)

## Conventions

- Folder prefix: `examples/stream/*`.
- Keep examples minimal; no secrets; use env interpolation from `.env`.

## Example Matrix

1. examples/stream/sse-browser

- Purpose: Show consuming workflow `/stream` via browser `EventSource`.
- Files:
  - `index.html` – plain JS with `EventSource` and basic UI.
  - `README.md` – how to run and wire `exec_id`.
- Demonstrates: SSE headers, event routing (`workflow_status`, `tool_call`, `complete`).
- Walkthrough:
  - Start API: `make dev`
  - Trigger a workflow; copy `exec_id`
  - Serve static: `python3 -m http.server 8080` and open `/?exec_id=...`

2. examples/stream/sse-node

- Purpose: Node consumer for agent/task streams (text + JSON cases).
- Files:
  - `app.mjs` – uses `eventsource` or fetch streaming polyfill.
  - `README.md` – commands and expected output.
- Demonstrates: text-only `llm_chunk` when no schema; structured JSON when schema present.
- Walkthrough:
  - `bun install eventsource`
  - `bun run app.mjs --url http://localhost:5001/api/v0/executions/agents/<exec_id>/stream`

## Minimal YAML Shapes

```yaml
# No special YAML required for streaming; examples may reuse existing hello-world workflow
# found in docs or examples to produce an execution id.
```

## Test & CI Coverage

- Add a smoke script under `examples/stream/` that exercises `/stream` and exits on `complete`.

## Runbooks per Example

- Prereqs: API on localhost; copy an `exec_id` from a triggered execution.
- Commands: see per-example README.

## Acceptance Criteria

- Browser and Node examples render events live and close on `complete` without errors.
- READMEs include exact commands and expected output snippets.
