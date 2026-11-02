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
overlaps: RT-session-runtime-selection-continuity; ET-web-session-prompt-runtime-and-create-navigation
---

Keyboard Enter and pointer confirmation are both part of the same interaction. Cancel returns to catalog search without changing the current runtime.

QA impact 2026-08-05 (review remediation): Enter and pointer confirmation now commit and return to
catalog mode, while cancel restores search focus. Reset for keyboard and pointer replay.

QA 2026-08-05 (review remediation): cancel restored catalog search focus; Enter and pointer commit
both returned to the open catalog with search focused and `Cursor Agent / Composer 2.5` selected.


QA impact 2026-11-01 (responsiveness audit): closed runtime selectors defer popup row projection.
Verify the selected model and valid advertised controls remain visible while closed; update the
catalog or favorites, reopen, and exercise browse, search, exact-ID entry, keyboard selection and
Escape focus restoration against the latest data. The existing runtime-selector component suite
owns focused coverage; the new check includes catalog updates while closed. This note does not
claim a fresh live-provider/browser walk; integrated runtime evidence belongs to the audit QA tail.
