---
id: ET-session-attachment-model-gate
area: ET
title: Gate attachments by negotiated agent capability
persona: Théo
journey: J-session-attachments
expected: Images and PDFs pass the composer gate and reach backend admission when the bound agent capability is unknown. Only an explicitly unsupported capability for the targeted runtime refuses image or PDF input; Markdown and plain text still dispatch as text blocks, and rejected drafts and previews remain available for correction.
entry_points: web session composer runtime selector; prompt API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-15: the attachment gate now distinguishes an unknown bound-agent capability from an explicit agent refusal. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
