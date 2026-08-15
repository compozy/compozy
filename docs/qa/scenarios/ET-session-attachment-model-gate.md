---
id: ET-session-attachment-model-gate
area: ET
title: Gate attachments by negotiated model capability
persona: Bruno
journey: J-session-attachments
expected: Images are refused without ACP image input and PDFs are refused without embedded context, while Markdown and plain text still dispatch as text blocks; rejected drafts and previews remain available for correction.
entry_points: web session composer runtime selector; prompt API
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412/01-image-capability-refused.png
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412.md
overlaps: ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
