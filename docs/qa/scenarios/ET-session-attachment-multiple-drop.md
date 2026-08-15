---
id: ET-session-attachment-multiple-drop
area: ET
title: Drop multiple session attachments
persona: Bruno
journey: J-session-attachments
expected: Dropping multiple supported files adds one preview per file, preserves their order, sends all refs in one prompt, and renders every attachment in the persisted transcript.
entry_points: web session composer drop target; session transcript
qa_status: pass
bug_ids: BUG-20260815-session-attachment-store-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 2603eed
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/08-multiple-files-dropped.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: ET-session-attachment-paste-reload
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
