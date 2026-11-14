---
id: ET-session-mcp-control-states
area: ET
title: The session composer MCP control shows ready, sign-in, approval and limited-by-parent states
persona: Nia
journey:
expected: The composer "MCP N" control counts usable servers, tints on attention, dots on pending change, and its popover shows CompozyOS rows (Ready · N tools, Needs sign-in + Sign in, Waiting for approval + Allow, Couldn't connect), the runtime section read-only with Manage in Settings, and a collapsed "Not available in this session" group in a narrowed child session.
entry_points: session composer MCP control (mcpControl slot); GET /api/workspaces/{ws}/sessions/{id}/mcp; session.mcp_changed
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-mcp-signin-from-session-control; ET-mcp-project-server-approval
---

Covers E2E-009 (US-010; `_uiux.md` S3/S4). Depends on S8 (composer control and transcript line).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.

Steps:

1. Session with: echo (ready), an OAuth server not signed in, the waiting `sentry` project fixture,
   and a broken stdio server. Open the control with Enter; Tab reaches every row action.
   Screenshot each state: Ready · N tools; Needs sign-in; Waiting for approval; Couldn't connect.
2. Add a server: the trigger shows the pending dot and the header "Applies on your next message"
   without reopening (driven by `session.mcp_changed`).
3. Spawn a child narrowed to `compozy` only; its control lists echo/sentry under "Not available in
   this session" with "Limited by the parent session", collapsed by default.
4. Runtime section: "Loaded by Claude Code" rows and "Manage in Settings"; with runtime_mcp off,
   "Off in CompozyOS sessions".
5. Light and dark screenshots compared with draft `web/src/drafts/session-mcp-control/`.

Pass: every listed state renders with its text label (never color alone).

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
