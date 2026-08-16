---
id: ET-session-attachment-multiple-drop
area: ET
title: Drop multiple session attachments
persona: Théo
journey: J-session-attachments
expected: Dropping multiple supported files adds one preview per file, preserves their order, sends all refs in one prompt, and renders every attachment in the persisted transcript.
entry_points: web session composer drop target; session transcript
qa_status: pass
bug_ids: BUG-20260815-session-attachment-store-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 2603eed
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/12-drop-overlay.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/13-multiple-drop-ready.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: ET-session-attachment-paste-reload
---
