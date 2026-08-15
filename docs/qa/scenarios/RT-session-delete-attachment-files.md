---
id: RT-session-delete-attachment-files
area: RT
title: Delete session attachment files
persona: Théo
journey: J-session-attachments
expected: Deleting a stopped session removes its catalog and history plus its workspace/session attachment directory; a failed catalog delete restores both, and deleting one workspace never removes another workspace's files.
entry_points: HTTP+UDS session delete; CLI session remove; COMPOZY_HOME session-attachments
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-014;RT-session-delete-owned-history
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
