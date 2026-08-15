---
id: ET-session-attachment-picker
area: ET
title: Choose a session attachment
persona: Bruno
journey: J-session-attachments
expected: The composer file picker accepts a supported file, shows its truthful name and removable preview, and sends it through the same stored attachment contract as paste and drop.
entry_points: web session composer file picker
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
