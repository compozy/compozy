# Claude native subagent fixture

`claude_agent_subagent.jsonl` contains JSON-RPC session/update frames emitted by
`@agentclientprotocol/claude-agent-acp` **0.88.0**'s exported `toAcpNotifications`.
It is a deterministic adapter-derived fixture, not a recording of a paid model run.

Source: `dist/acp-agent.js`, `toAcpNotifications` (line 8484),
`stampParentToolUseId` (line 501), and the adapter's tool-call renderer.
The installed source was read from the local npm cache. No provider login or
native ACP subagent-session capability was used.

Replay inputs, with one shared tool-use cache and `registerHooks: false`:

1. Assistant `tool_use`: id `toolu_agent`, name `Agent`, input
   `{description:"Review the diff (high effort)",prompt:"Review the diff",model:"sonnet-5.5"}`.
2. Assistant text `Inspecting the diff.`, option `parentToolUseId: "toolu_agent"`.
3. Assistant `tool_use`: id `toolu_read`, name `Read`, input
   `{file_path:"/workspace/README.md"}`, same parent option.
4. User `tool_result`: `tool_use_id: "toolu_read"`, content `Project notes`,
   `is_error: false`, same parent option.
5. User `tool_result`: `tool_use_id: "toolu_agent"`, content `No issues found.`,
   `is_error: false`, no parent option.

Each returned notification is wrapped as
`{jsonrpc:"2.0",method:"session/update",params:notification}`.
UT-040 pins attribution on all five frames. IT-020 replays them through the real
ACP mock subprocess into SQLite, changing only the session id to the mock's bound
session id. Future adapter upgrades should regenerate and review this fixture.

## Background launch fixture

`claude_agent_background_subagent.jsonl` holds the frames `@agentclientprotocol/claude-agent-acp`
**0.92.0** sends a client without the AIR or native subagent-session capabilities when Claude runs
its Agent tool in the background (the SDK default). The payload shapes and the launch receipt text
come from a real Claude session's ledger (2026-10-16, issue #766), sanitized: the agent id, output
path, and prompt are placeholders. The recorded input also carried `run_in_background: true`; the
fixture omits it so the provider marker alone identifies the background launch.

1. `tool_call` for `toolu_background` (`Agent`), title `Task`, empty input.
2. `tool_call_update` with the streamed input and the description as title.
3. The PostToolUse `tool_call_update`: `_meta.claudeCode.toolResponse` is the SDK's full
   `{status: "async_launched", isAsync: true, agentId, …}` response (`renderer.hookResult`, the
   non-AIR shape). The adapter sends it before the result.
4. `tool_call_update` `status: "completed"` whose content is only the launch receipt.
5. and 6. The background subagent's own Bash call and result, attributed with `parentToolUseId`
   after the receipt.

The adapter reports the background subagent's outcome only to the model (a `<task-notification>`),
and holds the prompt response until every background subagent of the turn has settled
(`turnAwaitingSubagents`). The ACP contract test pins that the receipt is persisted as an async
launch result; IT-020 replays the frames through the real ACP mock subprocess and pins that the row
runs until the turn's `done`.

## Task lifecycle fixture

`claude_agent_acp_0.98.0_task_lifecycle.jsonl` contains `_claude/sdkMessage`
extension notifications in the envelope `@agentclientprotocol/claude-agent-acp`
**0.98.0** sends when a session requests
`_meta.claudeCode.emitRawSDKMessages` (`dist/acp-agent.js` lines 3972-3977:
`extNotification("_claude/sdkMessage", {sessionId, message})`, emitted before the
adapter's own late-run filter at lines 4555-4700). Message shapes follow
`SDKTaskStartedMessage`, `SDKTaskProgressMessage`, `SDKTaskUpdatedMessage`, and
`SDKTaskNotificationMessage` in `@anthropic-ai/claude-agent-sdk/sdk.d.ts`. It is
hand-built from those declarations, not a recording of a paid model run.

The sequence covers a background shell task (ignored), a background `Explore`
agent through progress, a paused patch, a terminal `task_updated` patch without
summary, and its summary notification; a resumed run `0002` under a new tool
call; a late `stopped` notification of run `0001` (stale); a `killed` patch for
run `0002`; and a patch for a task that never started (uncorrelated).
