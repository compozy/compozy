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
overlaps: RT-014;RT-082;ET-web-sessions-catalog-modal
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
