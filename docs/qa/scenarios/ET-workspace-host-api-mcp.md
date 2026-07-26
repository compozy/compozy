---
id: ET-workspace-host-api-mcp
area: ET
title: Operate one AGH workspace from an external MCP client
persona: Ada
journey: J-operate-agh-from-mcp-client
expected: A trusted MCP client lists sessions and creates a task through a workspace-bound stdio relay, the effects match the native HTTP API, workspace B data stays unreachable, and loopback HTTP rejects missing or incorrect bearer tokens.
entry_points: agh mcp serve --workspace; MCP stdio client; MCP streamable HTTP client; native HTTP API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Start an isolated daemon with two registered workspaces. From a third-party MCP client, spawn
`agh mcp serve --workspace <workspace-a>` over stdio, list the advertised `agh_host__*` tools, create
one session and one task, and compare both with the native HTTP API. Confirm a relay bound to
workspace B cannot list workspace A's session.

Repeat with streamable HTTP on a loopback address. Require deterministic rejection without a bearer
token and with an incorrect token, then connect with the exact token sourced from the configured
environment variable. Confirm non-loopback startup is rejected and stopping the foreground process
leaves no relay listener or registered façade principal.

QA impact 2026-07-18: new workspace-bound MCP façade and public `agh mcp serve` CLI behavior.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: linked to J-operate-agh-from-mcp-client; settles US-010 (ADR-008).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Third-party client transcript (tool list, session list, task create) with the native HTTP reads
  proving the effects.
- Native-registry digest diff output (zero new `agh__*` IDs).
- Rejected tokenless and wrong-token non-stdio connections, the successful exact-token connection,
  and the workspace-B isolation probe.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-010-drive-agh-from-any-mcp-client
