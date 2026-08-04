---
id: ET-session-input-mcp-projection
area: ET
title: Operate pending session input through the workspace MCP relay
persona: Ada
journey: J-operate-compozy-from-mcp-client
expected: A standard MCP stdio client can start compozy mcp serve for one workspace, discover the exact sessions/inputs list, replace, cancel, and promote Host API tools with schemas, call the list tool through the workspace binding, and observe no new compozy__ native tool IDs.
entry_points: compozy mcp serve --workspace; official-SDK MCP stdio client; compozy tool list -o json
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: passed
fix_commits: final loop-lifecycle remediation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-lifecycle-mcp-inputs-20260804-044709-893107-lab/qa-artifacts/qa/mcp-projection-evidence.md
last_report: docs/qa/reports/2026-08-04-loop-lifecycle-mcp-inputs.md
overlaps: ET-workspace-host-api-mcp; RT-019
---

QA impact 2026-08-04: the durable session-input Host API methods became canonical in PR #304, but
the MCP projection decision table omitted all four methods and caused `compozy mcp serve` to fail
closed during startup. This scenario owns that cross-surface seam without replacing the broader
workspace-relay or busy-session journeys.

Passed 2026-08-04 in a fresh isolated lab. The official Go MCP SDK discovered all four tools with
object schemas, created a real workspace-bound session, and received `{"inputs":[]}` from the list
tool. An independent CLI read observed the same active session before cleanup. The native registry
reported 238 tools and zero `compozy_host__` IDs.

Walk the scenario through the built CLI and an unmodified official-SDK MCP client. Require all four
tool names, valid input schemas, a successful workspace-bound list call, a clean relay exit, and a
native registry read showing that no `compozy__*` IDs were added by the Host API projection.
