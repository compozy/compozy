---
id: RT-web-exact-model-id-entry
area: RT
title: Choose an exact provider model ID in the runtime selector
persona: Bruno
journey: J-17
expected: The exact-model action stays visible during catalog loading, opens a focused labelled field with an empty disabled confirmation, preserves an exact live Cursor ID, rejects a non-advertised Cursor alias without persisting it, and exposes the selected value through a public readback.
entry_points: web session composer Next prompt selector; web agent runtime selector; HTTP+UDS session runtime readback
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-web-network.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-web-config.toml;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-web-provider-settings.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md
last_report: docs/qa/reports/2026-08-13-issue-389-cursor-model.md
overlaps: RT-session-runtime-selection-continuity;ET-web-session-prompt-runtime-and-create-navigation
---

Keyboard Enter and pointer confirmation are both part of the same interaction. Cancel returns to catalog search without changing the current runtime.

QA impact 2026-08-05 (review remediation): Enter and pointer confirmation now commit and return to
catalog mode, while cancel restores search focus. Reset for keyboard and pointer replay.

QA 2026-08-05 (review remediation): cancel restored catalog search focus; Enter and pointer commit
both returned to the open catalog with search focused and `Cursor Agent / Composer 2.5` selected.
