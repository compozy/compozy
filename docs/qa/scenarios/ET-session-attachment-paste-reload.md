---
id: ET-session-attachment-paste-reload
area: ET
title: Paste, send, and reload a session image
persona: Bruno
journey: J-17
expected: Pasting one supported image into the session composer shows a removable preview; sending succeeds, and a cold reload renders the same attachment from persisted event metadata and the scoped file route.
entry_points: web session composer; session transcript reload
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-composer-text-entry
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
