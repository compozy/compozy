---
id: ET-mcp-project-server-approval
area: ET
title: A project-file MCP server waits for digest-bound approval and is never started before it
persona: Dora
journey: J-offer-runnable-capabilities
expected: A committed `.compozy/mcp.json` server reads Waiting for approval in Settings and in the session MCP control; it is never started, probed or contacted before approval; Allow binds the current definition digest and the next message calls it; editing the file returns it to waiting (Changed since you allowed it); revoke withdraws it on the next call; approvals do not leak to another workspace.
entry_points: web Settings → MCP servers (Project) → Allow; session composer MCP control → Allow; compozy mcp allow <server> --workspace <ws> [--digest|--revoke]; compozy__mcp_allow; PUT|DELETE /api/settings/mcp-servers/{name}/approval
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-cli-mcp-runtime-verbs; RT-mcp-stdio-session-reuse
---

Covers E2E-005 (US-005, Rules 1–3).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.
Project fixture: commit `.compozy/mcp.json` declaring `sentry` as a stdio wrapper around the echo
fixture that appends one line to `$LAB/starts.log` on every start.

Steps:

1. Open the project as a workspace; create a Claude session in it and send one message.
2. Settings → MCP servers → Project: `sentry` reads "Waiting for approval · Added by a file in this
   project" with Allow; header meta "1 waiting for approval". The composer MCP control shows the same.
   `starts.log` is empty (no discovery, probe or launch). Screenshots.
3. `compozy mcp list --workspace <ws> --compozy-only -o json`: `approval.state: "waiting"`,
   `reason: "new"`, `callable_in_sessions: false`; record `definition_digest`.
4. In the session, "call sentry echo": the error reads "sentry is waiting for approval in <ws>.
   Allow it in Settings → MCP servers."
5. An agent calling `compozy__mcp_allow` must receive an operator approval request in every
   permission mode, including `approve-all`; without an approval bridge the call fails closed.
   Click Allow in the MCP control. Send "call sentry echo". It succeeds; `starts.log` has one line.
6. Edit an argument in `.compozy/mcp.json`. The row returns to "Waiting for approval · Changed since
   you allowed it"; the old digest is refused:
   `compozy mcp allow sentry --workspace <ws> --digest <old>` → `mcp_approval_definition_changed`.
7. `compozy mcp allow sentry --workspace <ws>` (prints the definition, then "Allowed sentry in <ws>
   for profile default. Sessions can use it from their next message.").
8. `compozy mcp allow sentry --workspace <ws> --revoke`; the next call fails with `source_disabled`.
9. Open a second workspace with the same file: `sentry` is waiting there too (no cross-workspace leak).
10. `compozy mcp allow linear --workspace <ws>` for a personal server → `mcp_approval_not_required`.

Pass: no process start before approval; every message matches `_dx.md` Errors.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
