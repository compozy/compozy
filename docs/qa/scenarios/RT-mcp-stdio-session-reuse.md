---
id: RT-mcp-stdio-session-reuse
area: RT
title: Reuse one stdio MCP server process across discovery, status, and calls
persona: Dora
journey: J-offer-runnable-capabilities
expected: A slow-starting stdio MCP server launches once for repeated tool search, tool calls, and status probes; status reports ready; a crash backs off instead of relaunching per request and the status diagnostic carries the redacted stderr tail; daemon stop leaves no server process or isolated home behind.
entry_points: compozy tool search; compozy tool invoke mcp__<server>__<tool>; compozy__mcp_status; GET /api/settings/mcp-servers; compozy daemon stop
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-10-31-issue-1068-stdio-mcp-reuse.md
last_report: docs/qa/reports/2026-10-31-issue-1068-stdio-mcp-reuse.md
overlaps: RT-mcp-dead-recovery
---

Configure a user-scope stdio MCP server that takes about 3 seconds to start and logs each launch.
Run five tool searches, ten tool calls, and one `compozy__mcp_status` probe. Confirm the launch log
has one entry, every call succeeds, and status reports `ready` with the tool count instead of
`call_timed_out`. Confirm the server's `HOME` sits under `$COMPOZY_HOME/cache/mcp/homes/` and
`npm_config_cache` under `$COMPOZY_HOME/cache/mcp/npm`.

Make the server exit during a call. Immediately probe status and call again: no new launch occurs,
the call fails fast with `mcp_unreachable`, and the status diagnostic names the backoff wait and
includes the server's stderr tail with secrets redacted. After the backoff window, one call launches
the server again and succeeds.

Stop the daemon and confirm no server process remains, the pool homes directory is empty, and
`$TMPDIR` gained no `compozy-mcp-*` directories.

QA impact 2026-10-31 (#1068): new scenario. Walked on the branch build against base `37439a2c9`;
see the report for before/after launch counts and timings.
