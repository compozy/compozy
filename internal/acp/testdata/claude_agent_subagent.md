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
