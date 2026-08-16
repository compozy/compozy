---
id: ET-session-attachment-unsupported-type
area: ET
title: Refuse an unsupported attachment type
persona: Théo
journey: J-session-attachments
expected: A file whose detected MIME is outside the configured allowlist is refused before prompt dispatch; changing its extension does not bypass detection and no attachment ref is created.
entry_points: web session composer; attachment upload API
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/14-unsupported-file-error.png; qa-artifacts/qa/cli-unsupported.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: 
---
