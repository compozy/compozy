---
id: RT-web-exact-model-id-entry
area: RT
title: Choose an exact provider model ID in the runtime selector
persona: Bruno
journey: J-17
expected: The exact-model action stays visible during catalog loading, opens a focused labelled field with an empty disabled confirmation, preserves the typed ID exactly, and persists it for the selected provider through a public readback.
entry_points: web session composer Next prompt selector; web agent runtime selector; HTTP+UDS session runtime readback
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/issue-312-evidence.md;/Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/screenshots/issue-312-web-exact-model-empty.png;/Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/screenshots/issue-312-web-exact-model-filled.png;/Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/screenshots/issue-312-web-exact-model-selected.png;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/issue-312-review-evidence.md;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/screenshots/exact-empty.png;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/screenshots/exact-enter-committed.png;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/screenshots/exact-pointer-committed.png
last_report: docs/qa/reports/2026-08-05-issue-312-review-remediation.md
overlaps: RT-session-runtime-selection-continuity;ET-web-session-prompt-runtime-and-create-navigation
---

Keyboard Enter and pointer confirmation are both part of the same interaction. Cancel returns to catalog search without changing the current runtime.

QA impact 2026-08-05 (review remediation): Enter and pointer confirmation now commit and return to
catalog mode, while cancel restores search focus. Reset for keyboard and pointer replay.

QA 2026-08-05 (review remediation): cancel restored catalog search focus; Enter and pointer commit
both returned to the open catalog with search focused and `Cursor Agent / Composer 2.5` selected.
