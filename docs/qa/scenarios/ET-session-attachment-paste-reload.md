---
id: ET-session-attachment-paste-reload
area: ET
title: Paste, send, and reload a session image
persona: Théo
journey: J-session-attachments
expected: Pasting one supported image into the session composer shows a removable preview; sending succeeds, and a cold reload renders the same attachment from persisted event metadata and the scoped file route.
entry_points: web session composer; session transcript reload
qa_status: pass
bug_ids: BUG-20260815-session-attachment-store-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 2603eed
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/06-pasted-image-ready.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412/07-pasted-image-reloaded.png; qa-artifacts/qa/attachment-bytes-read.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: ET-web-session-composer-text-entry
---
