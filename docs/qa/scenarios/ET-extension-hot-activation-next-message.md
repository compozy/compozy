---
id: ET-extension-hot-activation-next-message
area: ET
title: An extension installed and enabled from Marketplace is callable in an open Codex session after one reload
persona: Théo
journey: J-offer-runnable-capabilities
expected: Installing and enabling a test extension with an MCP server or tool while a Codex session is open makes `ext ping` callable on the next message after exactly one reload; disabling the extension withdraws it on the next call.
entry_points: web Marketplace install; compozy extension install|enable|disable; compozy session mcp <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-mcp-hot-activation-next-message
---

Covers E2E-004 (US-003, US-006).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.
Extension fixture: a local test extension exposing a `ping` tool (the daemon integration suite's
extension helper, packed as a local-folder install).

Steps:

1. Create a Codex session; send one message; wait idle.
2. Install the test extension from Marketplace (or `compozy extension install <path>`) and enable it.
3. Send "call ext ping". One `session.mcp_reloaded` with `outcome: applied` and the extension in
   `added`; the call returns pong.
4. Disable the extension. Send "call ext ping again". The tool is withdrawn (absent or
   `source_disabled`).

Pass: one reload in step 3; the transcript keeps its history across the reload (Codex release +
`session/load`, re-verifying the Known Risk on plain `session/resume`).

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
