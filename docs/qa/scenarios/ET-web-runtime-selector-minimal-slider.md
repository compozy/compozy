---
id: ET-web-runtime-selector-minimal-slider
area: ET
title: Minimal runtime selector trigger and reasoning slider
persona: Sol
journey: J-17
expected: The session-composer "Next prompt" selector is ONE button (provider logo, model name, intensity meter, chevron — no provider name text, effort label, ⌘J badge, or dividers) that toggles the popup; provider rail and model rows remain minimal; the reasoning footer exposes only real levels, and drag, stop-label click, or track arrow keys commit the canonical effort for the next prompt snapshot.
entry_points: web session-composer runtime selector; agent create/settings runtime control; onboarding default-model step
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/05-session-runtime-selector.png;docs/qa/evidence/2026-07-30-session-runtime-selector/12-claude-max-selected.png;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-hover.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-keyboard-focus.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-needs-signin.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-model-glyph.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-disabled-search.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/verification-report.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-24-eng-143-runtime-selector.md
overlaps: RT-068;RT-072;RT-071;RT-064;RT-061
---

Added by the 2026-07-22 runtime-selector redesign (reference: docs/design/opendesign/_done/agents/provider-model-reasoning-selector.html). Trigger segments, deep-link focus routing, and the component-scoped ⌘J shortcut are deleted; the reasoning strip is replaced by the slider; `none` is no longer a selectable effort from the web UI.

QA impact 2026-07-25 (deep-review remediation): every slider thumb now has an accessible label,
including multi-thumb use. Flag only; the next QA cycle owns screen-reader and keyboard retesting.

QA impact 2026-08-25 (ENG-143): provider chips and cross-provider model glyphs now expose the
provider display name through the shared tooltip, while disabled provider chips stay silent during
search. Reset for targeted web hover and keyboard-focus verification.
