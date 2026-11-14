---
id: ET-web-runtime-selector-minimal-slider
area: ET
title: Minimal runtime selector trigger and reasoning slider
persona: Sol
journey: J-17
expected: The session-composer Next prompt selector remains one button with provider logo, model name, intensity meter, and chevron; it keeps provider rail and model rows minimal with no provider-name text, effort label, keyboard badge, or dividers; curated browsing remains distinct from full search; the reasoning footer exposes only valid levels, and drag, stop-label click, or track-arrow keys commit the selected model's advertised effort for the next prompt snapshot. It also exposes valid Fast controls, renders other advertised select or boolean options in Advanced, removes invalid selections after model changes, reacts to live descriptor updates, and disables unavailable controls for provider-managed runtimes.
entry_points: web session-composer runtime selector; agent create/settings runtime control; onboarding default-model step
qa_status: blocked-verify
bug_ids: BUG-20260906-runtime-selector-closing-focus
fix_status: fixed
retest_status: blocked
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/05-session-runtime-selector.png;docs/qa/evidence/2026-07-30-session-runtime-selector/12-claude-max-selected.png;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-hover.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-keyboard-focus.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-needs-signin.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-model-glyph.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/provider-tooltip-disabled-search.png;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/verification-report.md;/Users/pedronauck/dev/qa-labs/compozy-eng-143-runtime-selector-20260825-004835-671661-lab/qa-artifacts/qa/teardown.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-cursor-grok-catalog.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png;/Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/screenshots/cursor-grok-reasoning-fast.png
last_report: docs/qa/reports/2026-09-16-claude-fable-reasoning.md
overlaps: RT-068; RT-072; RT-071; RT-064; RT-061
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

QA re-walk 2026-09-06: selecting the runtime, closing with Escape, entering the composer and submitting now preserves composer focus through the popup exit. The provider/model-override browser journey asserts focus after the popup closes and passed with the combined repaired build. Existing model/effort persistence and ordinary Escape restoration also pass. See BUG-20260906-runtime-selector-closing-focus and `.cache/sessions-selector-root-integrated-green.log`.

QA 2026-09-09: pass for the changed catalog/reasoning journey. Fresh native discovery exposed Astra through Ultra, Luna through Max, Grok 4.5 through High and Grok 4.6 through Extra high. Saved runtime survived reload/restart. Native Cursor and Codex prompts completed; unsupported Cursor combinations returned 400 without changing the selection. See the current report for captures, exact runtime evidence and limits.


QA impact 2026-09-16, issue #655 (verification pending): use the installed Claude adapter in an
isolated daemon and select the exact Fable 5.1 identity from the full list and an existing recent or
favorite row. Compare cold discovery, persisted rows and a resumed session. Record the advertised
model-specific thought levels, choose an effort, submit a real prompt and confirm the effective
runtime plus ACP current value. Switch models while preserving a valid effort and clearing an invalid
one; exercise rapid selection, reload and queued-prompt snapshots. A model with unconfirmed metadata
must not claim provider-managed effort. Failed discovery must preserve prior rows and recover after
refresh. Keep distinct exact model versions and account/profile/workspace scope unchanged.

QA 2026-09-16, issue #655: Rendered full/recent/favorite Fable selection, valid/invalid effort switching, rapid-input persistence, and narrow viewport passed. Next-prompt provider acknowledgement remains blocked. The installed adapter independently confirmed the account Fable usage limit; a later application attempt also reported native authentication resolution failure. No selected intent was reported as applied. See the current report for public rendered evidence and the remaining recovery steps.

QA impact 2026-10-31, #1107: the session composer selector now applies same-provider changes to the live session and shows an apply note derived from the provider's runtime_live_apply capability; see RT-session-runtime-live-apply. Flag only.

QA impact 2026-11-14: the slider stops are now the intersection of the model's advertised
`reasoning_efforts` and every live ACP/config matrix list, so a model advertising `low/high/max`
no longer shows `medium` or other provider-wide levels. One shared rule
(`restrictReasoningEfforts`) caps the live session overlay, the catalog configuration matrix and the
latent session-derive options; an empty advertised list still lets ACP introduce levels, and a
matrix with no effort coordinate stays Fast-only. Automated coverage: `to-runtime-selector-options`
(intersection, advertised order, dedupe), `use-session-prompt-runtime` (catalog `low/high/max` vs
ACP `default/low/medium/high/max/future-effort` → exactly `low/high/max`, default collapsed),
`derive-active-session-options` and the `runtime-selector` renderer (`aria-valuemax=2`, Home/End).
The same ceiling now governs commits and defaults: a saved effort the intersection removed normalizes
to no effort even when a configuration matrix still lists it, and a default outside the final options
reads as absent. Reset for a live composer walk with a provider that publishes a wider ACP option than
its model profile, including switching models while an effort is selected.


QA impact 2026-11-11 (runtime selector polish): hovering a truncated model name or an OpenCode
upstream provider label reveals the full text through the shared tooltip without adding a tab stop
inside the listbox; provider chips now use a 28px target and replace the native rail scrollbar with
edge fades plus pointer-only paging chevrons. Verify hover on a long name and an OpenCode sub-label,
then overflow the rail with many providers and page it with the chevrons while keeping search focus.
Provider enable/disable and filter behavior are unchanged.

QA impact 2026-11-01 (responsiveness audit): closed runtime selectors defer popup row projection.
Verify the selected model and valid advertised controls remain visible while closed; update the
catalog or favorites, reopen, and exercise browse, search, exact-ID entry, keyboard selection and
Escape focus restoration against the latest data. The existing runtime-selector component suite
owns focused coverage; the new check includes catalog updates while closed. This note does not
claim a fresh live-provider/browser walk; integrated runtime evidence belongs to the audit QA tail.
