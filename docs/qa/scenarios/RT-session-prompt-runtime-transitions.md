---
id: RT-session-prompt-runtime-transitions
area: RT
title: Transition runtime at a prompt boundary
persona: Théo
journey: J-13
expected: A prompt captures its selected provider, model, reasoning effort, and speed; Compozy configures the live process or replaces it as required before dispatching that prompt, persists the transition, and restores the prior runtime if the transition fails.
entry_points: web session composer; POST /api/sessions/:sid/prompt over HTTP+UDS; CLI compozy session prompt
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md;docs/qa/evidence/2026-07-30-session-runtime-selector/07-sol-max-turn-complete.png;docs/qa/evidence/2026-07-30-session-runtime-selector/12-claude-max-selected.png
last_report: docs/qa/reports/2026-07-30-session-runtime-selector.md
overlaps: RT-018; RT-019; RT-061; RT-062
---

The canonical prompt runtime-transition scenario. It covers the runtime snapshot at dispatch, live reconfiguration versus process replacement, and rollback; RT-018 owns ordinary streaming, RT-019 owns busy-input modes, RT-061 owns reasoning order, and RT-062 owns rejected selections.
