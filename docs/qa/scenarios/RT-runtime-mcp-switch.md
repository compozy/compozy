---
id: RT-runtime-mcp-switch
area: RT
title: runtime_mcp off removes a runtime's own servers from sessions on the next message
persona: Dora
journey:
expected: For claude, copilot and qwen-code, turning the runtime switch off makes the runtime's own server absent from the session's reported MCP set on the next message while CompozyOS echo still works; a server added to the runtime file after off is also disabled after one reload; runtimes without a per-session switch show no switch and reject runtime_mcp=off with runtime_mcp_unsupported.
entry_points: Settings → MCP servers runtime group switch; Settings → Providers switch; compozy config set providers.<id>.runtime_mcp off; compozy session mcp <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-runtime-mcp-inventory-copy
---

Covers E2E-008 (US-009, Rule 10).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.
Fixtures: one own MCP server per runtime (`claude`, `copilot`, `qwen-code`) in its native file;
CompozyOS echo added in Settings.

Steps (per runtime in claude, copilot, qwen-code):

1. Create a session; ask the agent to list its MCP servers (`/mcp` or equivalent); the runtime's own
   server is present.
2. Turn the group switch off (or `compozy config set providers.<id>.runtime_mcp off`).
   `compozy session mcp <id>` shows the runtime rows as "Off in CompozyOS sessions" and pending.
3. Send a message: one `session.mcp_reloaded` with reason `runtime_mcp`; the agent's reported MCP
   set no longer has the runtime server; "call echo" still succeeds.
4. Add a second server to the runtime's native file; send a message: it is disabled too (Copilot/Qwen
   per-name flags recomputed; Claude strict config).
5. Turn the switch back on; next message restores them.

Then: `compozy config set providers.codex.runtime_mcp off` fails with `runtime_mcp_unsupported`;
Codex's group shows "Codex always loads its own servers" and no switch. With a managed
`managed-mcp.json` on the lab machine, Claude's group shows the note and no switch.

Open risk to verify here: Qwen Code's canonical provider ID is `qwen-code` while config validation
admits `off` only for `qwen` (see `_s9a-report.md`). Record whether `providers.qwen-code.runtime_mcp
= "off"` is accepted.

Pass: all three runtimes verified under ACP, or the failing runtime ships `CanDisableRuntimeMCP=false`
per the spec's fallback.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
