---
id: MS-web-settings-session-defaults
area: MS
title: Session defaults on General pick the provider, model and reasoning for new sessions
persona: Dora
journey: J-administer-runtime-settings
expected: Settings → General opens with Session defaults (Agent, the runtime selector, Smooth streaming) and a profile scope line. Picking provider + model + reasoning and saving writes `[defaults] provider`, `model` and `reasoning_effort` for the acting profile, reports restart-required, and after the restart a new session created without a runtime binds exactly that provider, model and reasoning. `/settings/defaults` lands on General. A config with only `[defaults] provider` resolves the provider's default model as before, and changing `defaults.provider` (Settings, `compozy config set`, install) removes the model and reasoning saved for the previous provider.
entry_points: web Settings → General; /settings/defaults; GET/PATCH /api/settings/persona; compozy config set defaults.provider|defaults.model|defaults.reasoning_effort; compozy session new
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/general-dark.png; https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/general-light.png; https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/03-selector-low.png; https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-814/05-saved.png
last_report: docs/qa/reports/2026-10-19-issue-814-session-defaults.md
overlaps: MS-web-settings-takeover-redesign; MS-settings-roles-panel
---

QA impact 2026-10-19 (#814): the Defaults page was retired into General. This scenario owns the
session-defaults save and resolution path; the retired route is a redirect only.
