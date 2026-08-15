---
id: ET-session-attachment-model-gate
area: ET
title: Gate attachments by negotiated agent capability
persona: Théo
journey: J-session-attachments
expected: Images and PDFs pass the composer gate and reach backend admission when the bound agent capability is unknown. Only an explicitly unsupported capability for the targeted runtime refuses image or PDF input; Markdown and plain text still dispatch as text blocks, and rejected drafts and previews remain available for correction.
entry_points: web session composer runtime selector; prompt API
qa_status: pass
bug_ids: BUG-20260815-session-window-runtime-selector-loop
fix_status: fixed
retest_status: pass
fix_commits: current-pr-head
evidence: docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/02-image-ready-no-false-warning.png; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/17-story-capability-gate.png; qa-artifacts/qa/api-session-caps.json
last_report: docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md
overlaps: ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-15: the attachment gate now distinguishes an unknown bound-agent capability from an explicit agent refusal. Flag only; the orchestrator's QA tail owns the persona walk and evidence.

QA verdict 2026-08-15: passed with a live GPT-5.6 Terra ACP binding. Unknown pre-bind capability admitted the draft to daemon truth, negotiated prompt_image true sent successfully without a false warning, and the explicit all-false Storybook state rendered the refusal gate.
