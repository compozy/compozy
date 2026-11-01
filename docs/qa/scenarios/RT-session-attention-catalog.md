---
id: RT-session-attention-catalog
area: RT
title: Discover sessions that need attention across workspaces
persona: Ada
journey: J-15
expected: CLI, HTTP, and UDS return the same workspace-scoped attention catalog, exact badge filters, stable attention ordering, and operator-wide summary totals across all profiles; operator-only cross-workspace catalog reads succeed while agent identity is confined to same-workspace interaction discovery.
entry_points: compozy session list --attention/--badge/--all-workspaces/--summary; compozy session interactions <session-id>; GET /api/sessions/attention-summary (all profiles) over HTTP and UDS; GET /api/workspaces/{workspace_id}/sessions/{session_id}/interactions over HTTP and UDS
qa_status: skipped
bug_ids: BUG-20261004-native-approval-timeout-pending
fix_status: fixed
retest_status: pending
fix_commits: 9f1457296
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-021; RT-session-clarification-roundtrip
---

Create sessions with permission, clarification, done, failed, and idle badges in at least two
workspaces. Compare counted pages, exact filters, stable cursors, summaries, structured status, and
sanitized interaction discovery across CLI, HTTP, and UDS. Confirm cross-workspace data never enters
a workspace-scoped page or cache, and agent identity receives the documented scope denial outside
same-workspace interaction discovery.

QA impact 2026-08-16: Task 01 added the canonical attention catalog, cross-workspace operator view,
exact summary, status badge, and interaction discovery surfaces. Flag only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA impact 2026-08-21: The profiles rebase made session-catalog scope explicit. The attention summary
remains an operator-wide aggregate across all profiles; the canonical scenario is reset for a
targeted CLI/API/UDS walk that verifies that cross-profile total and owner isolation.

2026-08-23 qa-impact (Profiles): attention now has a profile axis — the summary, badges, and badge
filters scope to the acting profile, alongside the workspace axis this row already exercises.
Already `untested`, so no reset was needed. When re-walking, seed attention-worthy sessions in two
profiles and confirm the counts an operator sees belong to the acting profile, that the two axes
compose rather than replace one another, and that an agent's identity stays confined to
same-workspace interaction discovery regardless of profile. Cross-profile totals are only available
through the explicit labeled aggregate, which `ET-profile-aggregate-owner-labels` owns.

Issue #595 acceptance: a self-directed Goal with a current action lease must not gain stale `loop_run` attention solely because no coordinator has completed yet. Expired leases and overdue waits still surface staleness; real quarantine, intervention, and approval remain attention even with fresh agent progress. Reread after recovery and terminal cancellation to confirm derived attention clears without editing stored badges.

Targeted #595 retest evidence and limits: [Goal lifecycle report](../reports/2026-09-10-issue-595-goal-lifecycle.md). This slice does not replace earlier evidence or claim an unrun full-scenario sweep.

QA impact 2026-10-04: a hosted native approval timed out while its interaction and waiting-for-auth
badge remained pending until daemon restart. BUG-20261004-native-approval-timeout-pending owns the
cancellation repair and focused replay; the full catalog, filter and cross-scope charter remains
untested until its remaining legs are walked.


QA 2026-10-04 targeted cancellation replay: CLI/UDS pending interactions and authorization attention
clear after the real 120-second native deadline without a daemon restart; a late decision reports
already-resolved/canceled and does not stop the target. A separate timely approval executes and
preserves the original owner. The full multi-workspace/catalog scenario remains untested; evidence:
docs/qa/evidence/2026-10-02-untested/native-cancel-replay-ended.json.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

Issue #756 acceptance: verdict-only system judge sessions do not contribute to attention totals, bell candidates or automatic attention toasts, including historical stopped/escalated judges. Automation/task/extension system sessions without verdict-only policy retain their attention behavior. Lifecycle events, explicit session audit reads and hooks remain available. Owning checks: TestManagerAttentionCatalogUsesCanonicalBadgesAcrossPages, TestSessionCatalogBroadcaster and TestPageSessionsVisibilityExclusion. Rendered before/after notification evidence is linked from PR #759.


QA impact 2026-10-17 (concurrent-session responsiveness): terminal permission
facets use the existing active-interaction partial index while retaining pending
status, visibility, profile/workspace scope and optimizer-selected join order.
Compare sparse and dense pending populations, including a narrow workspace among
many inactive interaction histories. Owning globaldb tests and before/after SQL
results are linked in [the responsiveness report](../reports/2026-10-17-responsiveness.md);
they do not replace outstanding public-surface scenario legs.

QA impact 2026-10-23 (Sessions Kanban backend; integrated re-walk pending): verify board-column
filters through CLI, HTTP/UDS and `compozy__session_list`. Seed 120 durable sessions with the last
20 in review; request two pages of 10 and check total 20, with facets still counting the complete
population. Change the column or child visibility on a cursor and expect rejection. Check
`--column blocked` exits 2 and the native tool rejects the same selector. Compare root-only counts
for `children=exclude` with `subagents=exclude`. Put direct children on later pages, archive one,
add a grandchild and a child in another profile: the parent's rollup includes only direct readable
non-archived children. An archived readable parent retains its title. Exercise active/durable
parity with waiting work and cached PRs, and ensure a changed repository origin cannot reuse facts
from the previous repo key. Package-scoped SQLite and CLI suites cover the backend contracts;
these checks do not claim the public-surface/UI journey has been re-walked.

Release regression 2026-10-24: after delegated subagents finish and their single
prioritized wake settles, the root is the only unseen-done item. Keeping a completed
subagent's ACP session alive must not leave the root in Waiting or hide it from the
Finished count. The unchanged prioritized-wake case in
`TestSubagentDaemonIntegration` verifies this against the real daemon/store;
see the merge-train v0.4.0 report for the scoped result.

B4 supervision impact: hold a permission on a Stays on session beyond the quiet
warning/stop interval. Confirm the supervision payload includes `held_permission`,
the session remains active without an inactivity warning, and answering removes
that signal. Orphaned permissions after restart must not count as live work.
Pending live QA; focused supervision tests are not a scenario execution receipt.
