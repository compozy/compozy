---
id: RT-session-delete-attachment-files
area: RT
title: Delete session attachment files
persona: Théo
journey: J-session-attachments
expected: Deleting a stopped session removes its catalog and history plus its workspace/session attachment directory; a failed catalog delete restores both, and deleting one workspace never removes another workspace's files.
entry_points: HTTP+UDS session delete; CLI session remove; COMPOZY_HOME session-attachments
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: qa-artifacts/qa/deletion-isolation.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: RT-014;RT-session-delete-owned-history
---
