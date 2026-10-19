---
id: ET-tool-result-artifact-recovery
area: ET
title: Recover one oversized tool result across public surfaces
persona: Rafa
journey: J-14
expected: An oversized post-hook redacted tool result keeps a truthful preview, opens as exact ordered bytes in Web, native tool, CLI, HTTP, and UDS, remains isolated to its workspace, survives daemon restart until deterministic retention removes it, and preserves a bounded partial result if persistence fails.
entry_points: Web session tool-result card; compozy__tool_artifact_read; compozy tool artifact read; GET /api/workspaces/:workspace_id/tool-artifacts/:artifact_id
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps:
---

Invoke a deterministic tool whose final post-hook, redacted result exceeds its effective result
budget and includes a multi-byte character across the 64 KiB page boundary. Confirm the transcript
keeps the bounded preview and one opaque artifact URI. Open every page in Web, then repeat through
the native reader, CLI, HTTP, and UDS; concatenate bytes in offset order and require the same
canonical JSON envelope on every surface.

Restart the daemon and read the artifact again. Confirm another workspace receives the same
not-found shape as an expired reference. Exercise count, byte, and age retention independently,
then inject one persistence failure and require a typed error with the bounded partial result and no
fabricated durable URI. Confirm the Web card preserves the preview through loading, not-found, and
retry states.

Through hosted MCP, call an oversized native tool (for example `compozy__session_events` with
`limit: 1000` on a session above the budget) from a real MCP client that validates output schemas
(Claude Code). Require a successful call whose `structuredContent` is the continuation envelope
(`truncated: true`, `truncated_from_bytes`, `preview`, `artifacts[].uri`, `read_tool`), then page
the artifact with `compozy__tool_artifact_read` to `eof` and match the retained bytes to the URI
digest. Confirm `compozy__session_events` and `compozy__session_history` without `limit` return
bounded pages with `page.has_more` and a continuation cursor.

QA impact 2026-10-19 (issue #816): hosted MCP results now always carry schema-valid
`structuredContent`, and native session reads page by default. Walked in an isolated lab with
Claude Code (`claude-sonnet-5-5`) through the hosted MCP server at the default 256 KiB budget:
`session_events` with `limit: 1000` and without `limit` returned the envelope (53 events,
`truncated_from_bytes` 327472) instead of the previous client-side "has an output schema but did
not return structured content" failure; ten 32 KiB `tool_artifact_read` pages reached `eof` with
327477 bytes whose SHA-256 matches the artifact URI; `session_history` with `limit: 1` returned
`page {"has_more":true,"limit":1,"next_before_sequence":52}`. Claude Code saves MCP output above its
token cap to a file, so a 64 KiB artifact page (about 88k base64 characters) does not stay inline;
32 KiB pages do. In the same lab the operator `compozy tool artifact read` returned
`tool_not_found` for these session-produced artifacts; that CLI leg remains unverified here.

QA impact 2026-07-19: new oversized tool-result retention, paging, management surfaces, and Web
viewer. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: settles defect D6 (TechSpec §3.1 tool-result offload) in the coverage
map; also proves US-003 EC-4 (artifact page-back on degraded resume).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Byte-identical page-back reads (offset-ordered concatenation) across Web, native, CLI, HTTP, and
  UDS for the oversized fixture, including the multi-byte page boundary.
- Post-restart read success, the cross-workspace not-found probe, and the three retention paths.
- The injected persistence failure returning a typed error with the bounded partial result and no
  fabricated durable URI.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-002-oversized-tool-results

2026-10-19 merge-train review follow-up: page forward through interleaved turns (one turn's
last event occurs after the next turn starts). Each page must include complete returned turns;
the next cursor must continue by turn start without skipping the overlapping turn. A real-SQLite
regression in `TestDaemonNativeTools` passes with bounded event reads and excludes late events
from turns that started before the cursor. Hosted-schema validation also covers a nested `$id`
resource with local references. Current-head CI owns the remaining integration gate.
