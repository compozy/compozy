---
id: RT-session-parent-provenance
area: RT
title: Session creation provenance recorded and queryable across CLI/HTTP/tool
persona: Ada
journey: J-cross-workspace-access
expected: compozy session new --parent <id> creates a user-type session whose lineage carries parent_session_id, root_session_id inherited from the parent's tree, and a server-computed spawn_depth, with no TTL, auto-stop, budget, or permission narrowing; a cross-workspace or missing parent is rejected with a deterministic error. POST /api/sessions accepts parent_session_id and infers the caller session when validated agent identity headers arrive without an explicit parent. compozy__session_create records the bound caller automatically (same workspace only). session list --parent <id> returns direct children, --root <id> returns the whole tree including the root, and the human table shows a Parent column; the same parent/root filters work on GET /api/sessions (HTTP+UDS) and compozy__session_list. Governed delegation lineage now comes from calls rather than a public spawn verb, and the reaper never touches user-type provenance sessions.
entry_points: compozy session new --agent reviewer --parent ses_01JBD7ZZAAAA and compozy session list --parent ses_01JBD7ZZAAAA --root ses_01JBD7ZZAAAA; HTTP and UDS POST /api/sessions with {"agent":"reviewer","parent_session_id":"ses_01JBD7ZZAAAA"}; compozy__session_create with {"agent":"reviewer","parent_session_id":"ses_01JBD7ZZAAAA"}; compozy__session_list with {"parent":"ses_01JBD7ZZAAAA","root":"ses_01JBD7ZZAAAA"}; a call-created child in the same lineage filters
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-sidebar-parent-20260806-212647-734931-lab/qa-artifacts/qa/journey-log.jsonl; /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: ET-session-command-catalog-parity; RT-session-list-row-actions; RT-session-spawn-removed
---

Added by the session parent-provenance backend (2026-08-06): provenance-light lineage on user
sessions across native tool, CLI, and HTTP/UDS create paths plus parent/root catalog filters.
Flag only — walk in the next QA cycle.

2026-08-06 walked in lab compozy-session-sidebar-parent-20260806-212647: CLI explicit --parent (parent/root/depth-1, user type, no governance fields), chained depth 2 under the original root, list --parent/--root filters (JSON + human Parent column), missing-parent and distinct-workspace rejections, HTTP explicit parent_session_id, and HTTP identity-header inference all passed live. Native-tool auto-link and reaper exclusion are covered by race-enabled suites (TestDaemonNativeTools, TestSpawnReaperSweep) since the lab carries no provider credentials. Verdict: pass.

Planning 2026-08-25 (agent-comms task_08): reset to `untested` because the row's stated boundary
changed. It claimed "governed spawn (`compozy spawn`) semantics are unchanged" against provenance-light
user lineage; that public verb no longer exists, and governed children now arrive through calls. The
2026-08-06 `pass` above is kept as history for the user-session half, which is unaffected. The re-walk
adds one comparison the old one could not make: a call-created child must appear in the same
`--parent`/`--root` lineage filters while still carrying its governance fields, so the two kinds of
lineage stay distinguishable. Re-walked by charter `CH-agent-comms-spawn-hard-cut-crossings`.
