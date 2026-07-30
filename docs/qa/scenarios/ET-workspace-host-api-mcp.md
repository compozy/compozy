---
id: ET-workspace-host-api-mcp
area: ET
title: Operate one Compozy workspace from an external MCP client
persona: Ada
journey: J-operate-compozy-from-mcp-client
expected: A trusted MCP client lists sessions and creates a task through a workspace-bound stdio relay, the effects match the native HTTP API, workspace B data stays unreachable, and loopback HTTP rejects missing or incorrect bearer tokens.
entry_points: compozy mcp serve; compozy mcp serve --workspace; MCP stdio client; MCP streamable HTTP client; native HTTP API
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps:
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: no session/task mutation, bearer rejection, or cross-workspace isolation proof was retained.

Start an isolated daemon with two registered workspaces. From a third-party MCP client, spawn
`compozy mcp serve --workspace <workspace-a>` over stdio, list the advertised `compozy_host__*` tools, create
one session and one task, and compare both with the native HTTP API. Confirm a relay bound to
workspace B cannot list workspace A's session.

Repeat with streamable HTTP on a loopback address. Require deterministic rejection without a bearer
token and with an incorrect token, then connect with the exact token sourced from the configured
environment variable. Confirm non-loopback startup is rejected and stopping the foreground process
leaves no relay listener or registered façade principal.

Launch once from inside workspace A without `--workspace` and confirm cwd inference binds A. Launch
again with an explicit workspace B override and confirm the façade rewrites every Host API request
to B and rejects conflicting caller input.

QA impact 2026-07-18: new workspace-bound MCP façade and public `compozy mcp serve` CLI behavior.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: linked to J-operate-compozy-from-mcp-client; settles US-010 (ADR-008).

QA impact 2026-07-28: MCP serve now uses the shared CLI resolution chain, and its projection consumes
the extension Host API workspace-binding authority. The scenario was already untested; no QA replay
ran in this implementation slice.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Third-party client transcript (tool list, session list, task create) with the native HTTP reads
  proving the effects.
- Native-registry digest diff output (zero new `compozy__*` IDs).
- Rejected tokenless and wrong-token non-stdio connections, the successful exact-token connection,
  and the workspace-B isolation probe.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-010-drive-compozy-from-any-mcp-client
