---
id: ET-web-runtime-selector-minimal-slider
area: ET
title: Minimal runtime selector trigger and reasoning slider
persona: Sol
journey: J-17
expected: The session-composer Next prompt selector remains one button with provider logo, model name, intensity meter, and chevron; it keeps provider rail and model rows minimal with no provider-name text, effort label, keyboard badge, or dividers; curated browsing remains distinct from full search; the reasoning footer exposes only valid levels, and drag, stop-label click, or track-arrow keys commit canonical effort for the next prompt snapshot. It also exposes valid Fast controls, renders other advertised select or boolean options in Advanced, removes invalid selections after model changes, reacts to live descriptor updates, and disables unavailable controls for provider-managed runtimes.
entry_points: web session-composer runtime selector; agent create/settings runtime control; onboarding default-model step
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/05-session-runtime-selector.png;docs/qa/evidence/2026-07-30-session-runtime-selector/12-claude-max-selected.png;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-hover.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-keyboard-focus.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-needs-signin.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-model-glyph.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-disabled-search.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/verification-report.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/teardown.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-cursor-grok-catalog.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png;/Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/screenshots/cursor-grok-reasoning-fast.png
last_report: docs/qa/reports/2026-08-28-cursor-onboarding-runtime-defaults.md
overlaps: RT-068;RT-072;RT-071;RT-064;RT-061
---

Added by the 2026-07-22 runtime-selector redesign (reference: docs/design/opendesign/_done/agents/provider-model-reasoning-selector.html). Trigger segments, deep-link focus routing, and the component-scoped ⌘J shortcut are deleted; the reasoning strip is replaced by the slider; `none` is no longer a selectable effort from the web UI.

QA impact 2026-07-25 (deep-review remediation): every slider thumb now has an accessible label,
including multi-thumb use. Flag only; the next QA cycle owns screen-reader and keyboard retesting.

QA impact 2026-08-25 (ENG-143): provider chips and cross-provider model glyphs now expose the
provider display name through the shared tooltip, while disabled provider chips stay silent during
search. Reset for targeted web hover and keyboard-focus verification.

QA impact 2026-08-27: the selector gained catalog-backed Fast and advanced ACP controls plus the
provider-managed state. Reset for keyboard, screen-reader, invalidation, and live-update walks.

QA 2026-08-28: pass. The shared selector exposed live Grok and Opus rows, valid reasoning stops,
Fast, typed Advanced options, and provider-managed disabling. Its canonical 98-test suite covered
keyboard, invalidation, catalog updates, and selection repair; the live composer retained High/Fast.

QA impact 2026-08-28: the onboarding owner now supplies the shared selector's `speed` and
`onSpeedChange` contract. Reset for a browser walk that proves Fast is visible, switchable, and
preserved when moving to the workspace step.

QA 2026-08-28: pass. The shared onboarding selector exposed an enabled Fast switch for Cursor Grok
4.6, retained it with Extra high reasoning, and reflected both values in its accessible label.
