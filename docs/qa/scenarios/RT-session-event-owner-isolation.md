---
id: RT-session-event-owner-isolation
area: RT
title: Refuse a session event store owned by another workspace
persona: Bruno
journey: J-operate-daemon-schema
expected: CLI, HTTP, and UDS session reads refuse an events.db owned by another session or workspace, and refuse a same-owner physical database family replaced during connection opening, without changing either SQLite family; the correctly owned sibling remains readable and restoring the matching complete directory recovers the original session.
entry_points: compozy session events <session-id>; compozy session history <session-id>; GET /api/workspaces/{workspace_id}/sessions/{session_id}/history
qa_status: pass
bug_ids: BUG-20261002-foreign-history-blocks-startup
fix_status: fixed
retest_status: pass
fix_commits: 3e35bf90; 212d9aea1
evidence: docs/qa/evidence/2026-10-02-untested/session-owner-fixed-refusal-summary.json; docs/qa/evidence/2026-10-02-untested/session-owner-restore-directory.json; docs/qa/evidence/2026-10-02-untested/session-owner-restored-alpha-http.json; docs/qa/evidence/2026-10-02-untested/session-owner-restored-beta-uds.json; docs/qa/evidence/2026-10-02-untested/session-owner-storage-race.log; docs/qa/evidence/2026-10-02-untested/session-owner-strict-audit-final.json; docs/qa/evidence/2026-10-02-untested/session-owner-teardown.json
last_report: docs/qa/reports/2026-10-02-untested.md
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

QA 2026-10-02: Fixed and replayed. A foreign retained history initially prevented the entire daemon
from starting. Boot now isolates typed identity refusals while preserving migration/cancellation
failures. CLI and both API transports refuse the foreign history and events; database, WAL, and SHM
hashes remain byte-identical. The healthy sibling stays readable. A stopped-daemon restoration of the
matching complete directory restores both original four-event histories through CLI, HTTP, and UDS.
The same-owner physical-open and ABA races are covered separately by the canonical
`TestOpenSessionDBReadOnly` storage fault-injection suite with real SQLite and race detection; this is
not a claim that the public persona walk controlled that sub-connection timing. The full SessionDB
suite and `make gate` pass. Strict targeted audit passes; teardown is clean with no lab survivor.
