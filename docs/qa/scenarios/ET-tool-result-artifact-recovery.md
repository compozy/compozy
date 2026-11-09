---
id: ET-tool-result-artifact-recovery
area: ET
title: Recover one oversized tool result across public surfaces
persona: Rafa
journey: J-14
expected: An oversized post-hook redacted tool result keeps a truthful preview, opens as exact ordered bytes in Web, native tool, CLI, HTTP, and UDS, remains isolated to its workspace or profile-owned User scope, survives daemon restart until deterministic retention removes it, and preserves a bounded partial result if persistence fails.
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

The same complete-turn invariant applies when walking backward with `next_before_sequence`:
late events of the selected turn must remain present even if they occur beyond that turn-start
cursor. The real-SQLite regression follows both cursor directions and checks the full old turn.

QA impact 2026-11-06 (issue #1316): repeat the native walk in a User session with
`compozy__skill_view` of `compozy` resource `references/runtime-operations.md` at the default
256 KiB result budget. Require one retained artifact and page `compozy__tool_artifact_read`
from offset zero through every `next_offset` until `eof`; the decoded envelope must contain
the complete structured resource and preserve display redactions. Repeat the read through
`GET /api/workspaces/@user/tool-artifacts/:artifact_id` and the operator CLI with `--workspace
@user`, using the producing profile. Workspace-to-User, User-to-workspace, and other-profile User
reads must return the same not-found shape as a missing artifact. Reopen the filesystem store
and exercise shared count, byte, and age retention over workspace and User artifacts.
Focused owning suites cover this behavior; previous provider-walk evidence above remains historical.

Workspace artifacts keep their existing workspace-only ownership and disk layout, including
artifacts produced before the upgrade from a non-default profile. No profile migration is applied
to these existing workspace artifacts; User artifacts have a separate profile-owned namespace.

Profile deletion removes that profile's User artifacts through its recoverable lifecycle
journal. A failed removal keeps the operation retryable; retry/recovery preserves other
profiles' User artifacts and all workspace artifacts. The artifact store is initialized
before boot replays profile operations.

QA impact 2026-11-09 (User-session event summaries): after the User walk above, read
`compozy logs --session <user-session-id> -o json`. The session's `tool_call`, `tool_result` and
`usage` summaries must be present with an empty workspace id, and the daemon log must not report
`observe: write event summary failed … event summary workspace_id is required`. A summary for a
session the catalog does not know still requires a workspace. Focused coverage:
`TestUserEventSummarySelection` (real SQLite) and `TestValidationHelpersAndPathUtilities`.
