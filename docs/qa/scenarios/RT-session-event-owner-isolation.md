---
id: RT-session-event-owner-isolation
area: RT
title: Refuse a session event store owned by another workspace
persona: Bruno
journey: J-operate-daemon-schema
expected: CLI, HTTP, and UDS session reads refuse an events.db owned by another session or workspace, and refuse a same-owner physical database family replaced during connection opening, without changing either SQLite family; the correctly owned sibling remains readable and restoring the matching complete directory recovers the original session.
entry_points: compozy session events <session-id>; compozy session history <session-id>; GET /api/workspaces/{workspace_id}/sessions/{session_id}/history
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits: 3e35bf90
evidence: /home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/evidence/session-owner/session-owner-walk.md; /home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/qa-audit-report.json; /home/pedronauck/dev/qa-labs/compozy-session-event-owner-isolation-20260803-080043-333009-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-03-session-event-owner-isolation.md
overlaps: RT-refuse-legacy-session-database
---

Prepare the mismatch only while the isolated daemon is stopped: preserve both complete session
directories, place one valid foreign `events.db` family under the other session directory, and record
the target family digest before the public reads. The walk must never edit owner or migration rows.

The refusal is complete only when every attempted public read fails, every target database/sidecar
digest remains unchanged, the source session remains readable, and restoring the matching complete
session directory makes the target session readable again.

QA result 2026-08-03: pass. A fresh isolated lab created sessions in two registered workspaces,
preserved both complete directories while the daemon was stopped, and placed beta's intact SQLite
family under alpha. CLI history/events, direct UDS history, HTTP history, and boot repair all refused
the exact foreign owner. The supplied `events.db`, WAL, and SHM hashes stayed byte-identical; beta
remained readable; restoring alpha's complete matching directory restored CLI, HTTP, and UDS reads.
The generic release-profile audit remains blocked by intentionally out-of-scope multi-actor,
provider, Web, task, disruption, artifact-reuse, and final-gate minimums; that wider profile does not
change this focused storage verdict.
Mandatory teardown recorded `clean=true` and zero surviving lab processes.

QA impact 2026-08-03: commit `3e35bf90` added an immutable physical `database_id` in append-only
SessionDB migration v5 and validates it on every actual read-only or writable SQLite connection. The
earlier pass remains valid historical proof for foreign owner refusal, but it predates same-owner
physical-family replacement detection. Reset to `untested`; replay the public owner/refusal walk in a
fresh isolated lab, add the v5 continuity case without editing owner/identity rows, and record a new
clean teardown.
