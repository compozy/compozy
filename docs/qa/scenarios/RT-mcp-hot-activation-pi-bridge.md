---
id: RT-mcp-hot-activation-pi-bridge
area: RT
title: A Pi session gets a newly added MCP server through the Pi bridge after one reload
persona: Théo
journey: J-offer-runnable-capabilities
expected: A Pi session running through the OpenRouter provider calls an echo server added after it started, on the next message, after one reload that writes a fresh Pi bridge config before session/load; a project server waiting for approval never appears in that bridge config.
entry_points: web Settings → MCP servers; compozy session mcp <id>; Pi bridge config under the session provider home
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-mcp-hot-activation-next-message
---

Covers E2E-006 (US-001 on Pi; Known Risk "Pi bridge refresh on reload").

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.

Steps:

1. Create a session with provider `openrouter` (Pi harness); send one message.
2. Add echo in Settings (Personal). Send "call echo". One `session.mcp_reloaded` (`applied` or
   `replayed`); the call succeeds through the Pi bridge.
3. Inspect the bridge config written for the new launch: it lists echo and does not list any
   waiting project server (repeat with the `sentry` fixture of ET-mcp-project-server-approval
   present but unapproved).

Pass: echo callable on the next message; bridge config fresh and free of waiting records.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
