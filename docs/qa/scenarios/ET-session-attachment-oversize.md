---
id: ET-session-attachment-oversize
area: ET
title: Refuse an oversized session attachment
persona: Théo
journey: J-session-attachments
expected: A file above session.attachments.max_file_bytes is refused before prompt dispatch with a clear limit error; the draft remains usable and no attachment ref or transcript message is created.
entry_points: web session composer; attachment upload API
qa_status: pass
bug_ids: BUG-20260815-session-attachment-cli-closed-pipe
fix_status: fixed
retest_status: pass
fix_commits: current-pr-head
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/16-packaged-oversize-413.png; qa-artifacts/qa/cli-oversize.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: 
---
