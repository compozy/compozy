---
id: ET-session-attachment-oversize
area: ET
title: Refuse an oversized session attachment
persona: Bruno
journey: J-session-attachments
expected: A file above session.attachments.max_file_bytes is refused before prompt dispatch with a clear limit error; the draft remains usable and no attachment ref or transcript message is created.
entry_points: web session composer; attachment upload API
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/04-oversize-refused.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: 
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
