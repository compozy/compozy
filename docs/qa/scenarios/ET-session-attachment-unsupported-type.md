---
id: ET-session-attachment-unsupported-type
area: ET
title: Refuse an unsupported attachment type
persona: Bruno
journey: J-session-attachments
expected: A file whose detected MIME is outside the configured allowlist is refused before prompt dispatch; changing its extension does not bypass detection and no attachment ref is created.
entry_points: web session composer; attachment upload API
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/05-unsupported-type-refused.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: 
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
