---
id: RT-cursor-agent-mode
area: RT
title: Start Cursor in a mode that can perform agent work
persona: Bruno
journey: J-17
expected: A new session navigates to its composer, whose next-prompt runtime selector exposes a supported Cursor operating mode; the selected prompt can create the requested artifact without an impossible switch instruction.
entry_points: web Start a new session; web session-composer runtime controls
qa_status: pass
bug_ids: BUG-20260713-cursor-agent-mode-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt;/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-session-runtime-selector-20260730-202203-576648-lab/cursor-runtime-proof.md;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md
last_report: docs/qa/reports/2026-09-09-model-catalog-reasoning.md
overlaps: RT-new-session-fast-feedback
---

The final assertion is a real filesystem artifact created by Cursor in the isolated workspace; an analysis-only response is not sufficient.

QA 2026-09-09: pass for the changed catalog/reasoning journey. Fresh native discovery exposed Astra through Ultra, Luna through Max, Grok 4.5 through High and Grok 4.6 through Extra high. Saved runtime survived reload/restart. Native Cursor and Codex prompts completed; unsupported Cursor combinations returned 400 without changing the selection. See the current report for captures, exact runtime evidence and limits.
