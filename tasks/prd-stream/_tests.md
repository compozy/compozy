# Tests Plan — Executions Streaming (/stream)

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc` and project rules.
- Prefer deterministic, isolated tests; use miniredis for Pub/Sub.

## Coverage Matrix (PRD → Tests)

- Workflow durable events → SSE handler emits only new events, supports Last-Event-ID.
- Agent/Task structured mode → JSON events; resume supported; closes on terminal.
- Agent/Task text mode → forwards `llm_chunk` lines from Redis Pub/Sub; closes on terminal.
- Heartbeats → emits every 15s; does not break client parsers.
- Headers → correct SSE headers and CORS when enabled.

## Unit Tests

- engine/infra/server/router/sse_test.go
  - Should set SSE headers and flush
  - Should send heartbeat frames
- engine/workflow/router/stream_test.go
  - Should emit only new events (cursor)
  - Should honor Last-Event-ID and resume
  - Should close on completed status
- engine/agent/router/stream_test.go
  - Structured: polls repo; emits JSON; closes on terminal
  - Text: subscribes to Redis; emits `llm_chunk`; tolerates empty backlog
- engine/task/router/stream_test.go
  - Mirrors agent tests for task execs

## Integration Tests

- With miniredis:
  - Publish sample chunks to `stream:tokens:<exec_id>`; verify SSE stream forwards lines in order.
- With workflow repo stub + Temporal client mock:
  - Provide Query response snapshots and assert only deltas are sent.

## Fixtures & Testdata

- `engine/agent/router/testdata/` – sample JSON events, text samples
- `engine/task/router/testdata/` – same as above

## Mocks & Stubs

- Temporal `QueryWorkflow` mock
- Task/Workflow repository stubs
- Redis Pub/Sub: miniredis client

## API Contract Assertions

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
- Optional: `X-Accel-Buffering: no`

## Observability Assertions

- Log a connect/disconnect with exec_id and last id
- Metrics counters for active streams

## Performance & Limits

- Ensure handlers flush per event; no excessive buffering
- Bound runtime per connection (configurable)

## Exit Criteria

- All new tests pass locally.
- CI green for linters and tests.
