---
id: RT-mcp-hot-activation-next-message
area: RT
title: An MCP server added in Settings is callable in an open Claude session on the next message
persona: Théo
journey: J-offer-runnable-capabilities
expected: With the default policy, a personal echo server added in Settings while a Claude session is idle shows `applies_next_message`, then the next message calls `echo` in that turn; the transcript shows at most one "MCP servers updated" line and `compozy session mcp` reports `echo` Ready afterwards. Removing the server withdraws the tool on the next call.
entry_points: web Settings → MCP servers → Add MCP server; session composer MCP control; compozy session mcp <id>; GET /api/workspaces/{ws}/sessions/{id}/mcp
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-mcp-hot-activation-pi-bridge; ET-extension-hot-activation-next-message
---

Covers E2E-001 (US-001).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.

Steps:

1. Create a Claude session in the lab workspace and send one message so it binds. Wait until idle.
2. `compozy session mcp <id> -o json`: record `usable` and that no `echo` row exists.
3. Settings → MCP servers → Personal → Add MCP server: add the echo fixture as `echo`. Toast reads
   "Open sessions use it from their next message."
4. `compozy session mcp <id>`: `echo` reads "Applies on your next message"; the composer MCP control
   shows the pending dot. Screenshot.
5. Send "call echo with E2E-001". The agent calls `mcp__…echo` in that turn with the text echoed back.
6. Transcript: zero or one "MCP servers updated · echo added" line (Claude may refresh through
   `tools/list_changed` without a reload). `session_events` contains at most one `session.mcp_reloaded`.
7. `compozy session mcp <id> -o json`: `echo` state `ready`, `pending_reload: false`.
8. Delete `echo` in Settings; send "call echo again". The call fails or the tool is absent; the error
   names `echo was removed from CompozyOS` when the agent calls a cached name.

Pass: steps 5–8 hold without stopping, resuming, forking or recreating the session.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md

QA impact 2026-11-17 (#1785): repeat the open-session flow with an OAuth echo fixture. Cross access
token expiry with a usable refresh token; the hosted catalog digest and proxy tools stay present,
and the next call renews and succeeds without reloading or signing in. A temporary renewal failure
keeps the catalog present while the call reports temporary unavailability. A definitive refresh
rejection withdraws the tools from that same proxy; sign-in restores them. Expiry without a refresh
token, missing credentials, revoked consent and session-budget exclusions must remain hidden.

QA impact 2026-11-17 (#1794, daemon CPU hot paths):
Unchanged MCP listings share a three-second discovery floor; explicit checks bypass it. Project approval still invalidates hosted projections immediately and next-message activation remains required. The targeted real-transport walk covers pending-to-approved tool visibility without restart.
See [targeted runtime evidence](../reports/2026-11-17-daemon-cpu-hot-paths.md).

QA impact 2026-11-18 ([#1850](https://github.com/compozy/compozy/issues/1850))

Bind 18 sessions to a five-server catalog of about 1,300 tools. Change one fixture and send
tools/list_changed. Verify its catalog is listed once and delivered to the sessions, while the
other upstream list counters remain unchanged.

Repeat with a fixture that has no live subscription and advertises a short `ttlMs`. Keep the
sessions idle and check that the fixture gets one shared refresh per interval of at least 60 seconds.
Use a longer `ttlMs` and verify that the refresh waits at least that long. An unchanged listing
keeps the existing tool projection. Release the last authorized bind and verify that periodic
refresh stops. An explicit check still contacts the fixture immediately.

For #1850, open one session with two ready fixtures and send a message. Turn one server off,
keep that session open, and send the next message. The off fixture contributes zero tools and the
other fixture remains usable. Turn the same server on and send another message in that session.
Its tools return using the same definition and approval, with no delete or re-import.
See the [off-switch walk](../reports/2026-11-18-mcp-server-off-switch.md).

Budgeted variant (#1869): open a Claude session with a small fixture and a fixture above the
threshold. Search for a deferred tool and call it in that same prompt. Refresh the large fixture's
catalog and verify the activated tool stays listed. Turn the large fixture off and verify its tools
are removed before the next call. Turn it back on and verify search can activate its tools again.
The full projected catalog drives Ready and pending status throughout the walk.
