---
id: ET-web-runtime-selector-minimal-slider
area: ET
title: Minimal runtime selector trigger and reasoning slider
persona: Sol
journey: J-17
expected: Closed selector is ONE button (provider logo, model name, intensity meter, chevron — no provider name text, no effort label, no ⌘J badge, no dividers) that toggles the popup; provider rail items have visible vertical spacing; model rows are single-line (bare provider icon without a boxed background, name, faint brain icon on reasoning-capable models, no context/cost/tools/levels chips); the selected row uses the neutral gray row tint plus a check, never the accent tint; the reasoning footer is a range slider whose stops are only the model's real levels (no None, no Default stop) with the model's default effort preselected while the wire value stays empty; drag, stop-label click, and track arrow keys all commit explicit canonical efforts.
entry_points: web session-create runtime selector; agent create/settings runtime control; onboarding default-model step
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-068;RT-072;RT-071;RT-064;RT-061
---

Added by the 2026-07-22 runtime-selector redesign (reference: docs/design/opendesign/_done/agents/provider-model-reasoning-selector.html). Trigger segments, deep-link focus routing, and the component-scoped ⌘J shortcut are deleted; the reasoning strip is replaced by the slider; `none` is no longer a selectable effort from the web UI.

QA impact 2026-07-25 (deep-review remediation): every slider thumb now has an accessible label,
including multi-thumb use. Flag only; the next QA cycle owns screen-reader and keyboard retesting.
