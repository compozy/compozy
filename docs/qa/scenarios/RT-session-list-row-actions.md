---
id: RT-session-list-row-actions
area: RT
title: Manage sessions from each catalog row
persona: Cora
journey: J-archive-session-without-deleting
expected: Every eligible session row exposes an accessible three-dot menu whose Archive, Unarchive, and Delete actions operate without first opening the session; opening or using the menu does not trigger row navigation, destructive deletion keeps its confirmation, archived rows render in a separate catalog section, and pointer, keyboard, compact, and desktop interactions remain usable.
entry_points: Web agent session list; global session catalog; session row actions menu
qa_status: pass
bug_ids: BUG-20260805-session-delete-dialog-disappears
fix_status: fixed
retest_status: pass
fix_commits: PR-309-coderabbit-remediation
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/journey-log.jsonl;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/session-catalog-desktop.png;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/agent-sessions-narrow.png;/Users/pedronauck/dev/qa-labs/compozy-session-archive-review-20260805-060247-848289-lab/qa-artifacts/qa/journey-log.jsonl;docs/qa/evidence/2026-08-05-session-archive-coderabbit/CH-archive-session-catalog-delete-pending-fixed.png;docs/qa/evidence/2026-08-05-session-archive-coderabbit/session-catalog-visual.png
last_report: docs/qa/reports/2026-08-05-session-archive-coderabbit.md
overlaps: RT-014; RT-082; ET-web-sessions-catalog-modal
---

The menu must use the shared UI primitives and keep Archive unavailable when runtime truth says the
session is not stopped. Archived rows offer Unarchive and retain direct navigation to the session.

QA completion 2026-08-05: both catalogs exposed the correct lifecycle action for active, stopped,
and archived rows. Escape returned focus to the trigger, row navigation remained independent, and
desktop plus narrow captures showed no clipped or blocked action targets.

QA impact 2026-08-05: the shared delete confirmation now rejects Escape and outside-close requests
while deletion is pending. Reset for focused re-verification in both session catalogs.

QA finding 2026-08-05: the shared lifecycle hook still unmounted the confirmation before the DELETE
request settled. The global catalog also closed, leaving no pending feedback. See
`BUG-20260805-session-delete-dialog-disappears`.

QA completion 2026-08-05: a fresh Cora replay held DELETE open and confirmed the shared confirmation
retained its target, pending state, and dismissal guard. A real follow-up deletion removed the row
only after success while leaving the catalog open.

## Profile-bound session regression (#783)

From All profiles, operate on a stopped session owned by a nondefault profile. Rename,
archive, and restore it without switching the selected profile; inspect its deep link
and subagent roster. Confirm the request uses the row owner for mutations, the roster
returns successfully, and a terminal 404/410 does not cause recurring control polls.
A failed cold roster page must stop automatic pagination until an explicit retry.
Record the session slice separately from this scenario's broader coverage in
`docs/qa/reports/2026-10-17-issue-783-profile-sessions.md`.


Issue #832 extension verified on 2026-10-19: focused E2E-032 proves owner-grouped folders, cross-profile drop rejection, owner-bound rename and reload persistence. Real Codex QA proves runtime/agent metadata for root and child rows, including never-bound, idle-released and stopped/unbound after daemon restart. Ordering and collapsed-folder navigation are covered by the owning unit suites. Evidence and exact scope: [issue #832 report](../reports/2026-10-19-issue-832-session-list.md).

QA impact 2026-10-22 (issue #904): both row menus add Copy session ID and Copy title, and a
nested provenance child adds Make top-level. Walked in ET-web-session-list-aggregate-parity.

QA impact 2026-10-24 (issue #952): the session window head no longer carries a per-session `…`
menu. Rename, Continue, Fork, and Delete were already row verbs; Clear conversation, the only
verb unique to the head menu, is now a row verb offered only on the row of the session that
window has open (disabled while a turn runs or nothing is left to clear). Sessions Kanban cards
carry the same row menu on right-click and behind a hover/focus `…`. The list toolbar splits
Sort and Group by into two icons. Walked in ET-web-sessions-board-card-actions-group-sort.

QA impact 2026-10-30 (#1051): verify User-owned rows whose `workspace_id` is null,
absent, or empty. From User and All projects, rename, stop, archive, restore, and
delete them through the catalog. Keep a delete confirmation open while switching
to a project; confirmation must still target `@user` with the captured profile.
Project-owned targets must retain their own workspace. Check mixed User/project
bulk delete and archive, and ensure unrelated project detail caches remain intact.
Focused hook coverage passes; browser verification is pending.
