# ACP mock fixtures

Fixtures use version `2`, named `agents`, and prompt-matched `turns` containing ordered
`steps`. Use typed `match.turn_source` and `match.user_text` to select a turn. The mock
driver runs as an ACP agent subprocess; see `testdata/` for complete fixtures.

## Calling hosted native tools

`native_tool_call` executes a real native tool from the agent process through the
`compozy-hosted-tools` stdio MCP server injected in ACP `session/new` or `session/load`.
It uses MCP `tools/call`; no operator API invocation or synthetic tool response is involved.

```json
{
  "kind": "native_tool_call",
  "tool_id": "compozy__subagent_delegate",
  "tool_call_id": "delegate-child",
  "raw_input": {
    "task": "child work",
    "title": "Fixture delegation",
    "target": { "agent": "subagent-worker" }
  }
}
```

- `tool_id` must identify a `compozy__` native tool. `tool_call_id` is required and is
  forwarded as MCP `_meta.toolCallId` and used by both ACP session updates.
- `raw_input` is a required JSON object; use `{}` for a tool without arguments.
- The ACP `tool_call` starts in progress, followed by `tool_call_update` with the real
  result as `rawOutput` (structured content when available, otherwise MCP content).
  MCP text content is also included. The default title is `tool_id`; `title` can override it.
- Successful calls finish as `completed`; MCP error results finish as `failed`.
  Transport failures emit a failed update and fail the prompt. Prompt cancellation cancels
  the pending MCP request.
- `status`, `raw_output`, and `content_text` cannot script a native result. Use the existing
  `tool_call` step when a scenario intentionally needs synthetic ACP events.
- Agents with this step connect their hosted MCP server at session initialization and reuse
  that connection across calls and turns. This preserves the single-use bind nonce. The
  connection closes on session close or driver shutdown. Fixtures without native calls
  continue to leave the injected server untouched.

`testdata/native_tool_delegate_fixture.json` supplies `subagent-delegator` and
`subagent-worker`. Register both under those agent names, prompt the parent with
`delegate child work`, and inspect its transcript for `delegate-child`. The parent discovers
capabilities, delegates asynchronously, and ends its turn; the worker answers and the
parent accepts the synthetic completion wake. This fixture can be used by browser E2E.
