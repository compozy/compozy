---
id: ET-mcp-signin-from-session-control
area: ET
title: Sign in to an OAuth MCP server from the session MCP control and use it on the next message
persona: Théo
journey: J-mcp-authorize-repair
expected: An OAuth echo server added while a Codex session is open reads Needs sign-in in the composer MCP control; Sign in opens the existing authorize dialog; after authenticated && token_present the next message calls echo through one reload.
entry_points: session composer MCP control → Sign in; MCPAuthorizeDialog; compozy mcp auth login <name>; compozy session mcp <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-mcp-authorize; ET-cli-mcp-authorize
---

Covers E2E-003 (US-002).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.
OAuth fixture: the fake authorization server `web/e2e/fixtures/mcp-auth-server.ts` (or
`internal/testutil/mcpfixture` OAuth profile) fronting the echo server.

Steps:

1. Create a Codex session; send one message; wait idle.
2. Add the OAuth echo server in Settings (Personal, HTTP, no headers).
3. Composer MCP control: the row reads "Needs sign-in" with a Sign in button; the trigger is tinted
   (attention). `compozy session mcp <id>` shows reason "Sign in to echo in CompozyOS". Screenshot.
4. Click Sign in in the control. Complete the authorization. The dialog confirms only on
   `authenticated && token_present`.
5. Send "call echo with E2E-003". The transcript shows one "MCP servers updated · echo added"
   line (Codex reloads its native session) and the call succeeds.
6. `compozy mcp auth logout echo`; send "call echo". The tool is gone; a cached call returns
   `mcp_auth_required` "echo needs sign-in. Sign in to echo in CompozyOS (Settings → MCP servers)."

Pass: steps 3–6 in one session, no restart.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
