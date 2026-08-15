---
id: ET-session-attachment-multiple-drop
area: ET
title: Drop multiple session attachments
persona: Bruno
journey: J-session-attachments
expected: Dropping multiple supported files adds one preview per file, preserves their order, sends all refs in one prompt, and renders every attachment in the persisted transcript.
entry_points: web session composer drop target; session transcript
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-attachment-paste-reload
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
