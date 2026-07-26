---
id: RT-mcp-dead-recovery
area: RT
title: Diagnose and automatically recover a dead MCP server
persona: Dora
journey: J-offer-runnable-capabilities
expected: Five confirmed permanent failures mark only the affected workspace MCP server dead; settings, status, doctor, Web, native status, and same-lifetime retained tool descriptors expose a redacted reason; ordinary attempts are suppressed; one due probe succeeds and clears the mark without a daemon restart.
entry_points: Web /mcp; GET /api/settings/mcp-servers; GET /api/status; agh status; agh doctor --only mcp; agh__mcp_status; agh__tool_info; MCP tool discovery
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-046
---

Configure the same stdio MCP server in two workspaces. Terminate the first workspace's server and
drive five confirmed permanent discovery failures. Confirm only that workspace reports `dead` and
`backend_dead`, its last-known tools remain diagnosable but unavailable, and repeated access inside
the 60-second window does not relaunch the process. Repair the server, wait for the recovery window,
then trigger one runtime access and confirm the server returns to ready without restarting AGH.
Confirm doctor observes the mark without consuming the recovery attempt and that no manual revive
control appears.

QA impact 2026-07-15: new workspace-scoped dead-runtime suppression, diagnostics, Web status, and
automatic recovery behavior. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Dora and linked to J-offer-runnable-capabilities;
settles the dead-entity half of US-011 (ADR-010 §5, Safety Invariant 20).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Dead-mark transition log with the probe-cadence delta to the low-frequency lane (measured).
- Unavailable-with-reason captures across status, doctor, Web, and native surfaces; workspace-B
  isolation probe (its identical sidecar unaffected).
- The revive capture: one due probe success auto-clears the mark without daemon restart; a
  transient-timeout run that never marks dead.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-011-only-runnable-skills-are-offered-dead-sidecars-self-recover
