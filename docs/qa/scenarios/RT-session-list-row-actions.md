---
id: RT-session-list-row-actions
area: RT
title: Manage sessions from each catalog row
persona: Cora
journey: J-archive-session-without-deleting
expected: Every eligible session row exposes an accessible three-dot menu whose Archive, Unarchive, and Delete actions operate without first opening the session; opening or using the menu does not trigger row navigation, destructive deletion keeps its confirmation, archived rows render in a separate catalog section, and pointer, keyboard, compact, and desktop interactions remain usable.
entry_points: Web agent session list; global session catalog; session row actions menu
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-014;RT-082;ET-web-sessions-catalog-modal
---

The menu must use the shared UI primitives and keep Archive unavailable when runtime truth says the
session is not stopped. Archived rows offer Unarchive and retain direct navigation to the session.
