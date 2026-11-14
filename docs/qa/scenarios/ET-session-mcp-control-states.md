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

Backend regression prerequisite: in the default profile, both a User session and a project session
must expose their MCP inventory. `GET /api/workspaces/{ws}/sessions/{id}` returns 200 with
`mcp_summary` for the project session; reading MCP state must not request the reserved default profile
as a profile-scoped Settings overlay. The canonical real-daemon check is
`TestDaemonSessionMCPReadSurfaces`; `TestSessionMCPReadModel` covers both scope selectors.

Repeat the session-detail and `/mcp` reads with a custom ACP provider (the isolated `acpmock`
provider is the automated fixture), with a configured CompozyOS MCP source, before and after its
first prompt. Both return 200, session detail
includes `mcp_summary`, and the MCP runtime section has an empty server list with the provider ID as
its display name. Unsupported or empty session provider identity must not scan another runtime's
native files. Explicit unknown runtime filters on the Settings inventory route still fail validation.
The custom-provider branch of `TestDaemonSessionMCPReadSurfaces` verifies the HTTP behavior with a
real daemon and ACP process; `TestRuntimeMCPStartup` and `TestMCPReadSnapshotCatalogChanges` cover the
empty identity and configured native-runtime alias boundaries.

Steps:

1. Session with: echo (ready), an OAuth server not signed in, the waiting `sentry` project fixture,
   and a broken stdio server. Open the control with Enter; Tab reaches every row action.
   Screenshot each state: Ready · N tools; Needs sign-in; Waiting for approval; Couldn't connect.
2. Add a server: the trigger shows the pending dot and the header "Applies on your next message"
   without reopening (driven by `session.mcp_changed`). Send a message to apply the change; after its
   terminal output, immediately send another. Both messages complete normally, the reload outcome
   remains in the durable session log, and the second message is not refused as already in progress.
3. Spawn a child narrowed to `compozy` only; its control lists echo/sentry under "Not available in
   this session" with "Limited by the parent session", collapsed by default.
4. Runtime section: "Loaded by Claude Code" rows and "Manage in Settings"; with runtime_mcp off,
   "Off in CompozyOS sessions".
5. Light and dark screenshots compared with draft `web/src/drafts/session-mcp-control/`.

Pass: every listed state renders with its text label (never color alone).

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
