---
id: MS-web-settings-takeover-redesign
area: MS
title: Settings takeover shell with srow pages and save models
persona: Dora
journey: J-administer-runtime-settings
expected: The settings window renders the 264px takeover sidebar (Back to app closes the window; search with `/` shortcut filters sections; Workspace/Runtime/Personal/System groups; runtime foot naming CompozyOS, never "daemon") collapsing to a chip strip under 56rem. Section labels read Remote access, Notifications, and Diagnostics, while their slugs stay `gateway`, `attention`, and `observability`, and searching the retired word still finds the renamed section. Pages use one-decision srows with consequence sentences, at most one Advanced fold per page, and choice cards with neutral selection. Draft pages show the floating save bar only when dirty/saving/error and flash "Saved" after a clean save; restart-needed changes surface the typed restart notice.
entry_points: web settings window (General, Memory, Automation, Skills, Hooks, Extensions, Network, Diagnostics, Notifications, Remote access)
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: MS-026; MS-037; ET-012; ET-044; ET-045
---

2026-08-20 retry: skipped by explicit user instruction. No settings search, save, or error path was walked.

Introduced by the opendesign settings redesign (docs/design/opendesign/settings/settings-general.html and siblings, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-S1/.

2026-08-20 qa-impact: reset by the normie-friendly UI foundation pass. `settings/lib/sections.ts`
renamed the `operator` group `Operator` → **Personal** and three section labels: `Gateway` →
**Remote access**, `Attention` → **Notifications**, `Observability` → **Diagnostics**. Slugs, routes,
and config keys are unchanged — these are UI aliases.

The retired words were folded into each section's `keywords` string on purpose, so the search field
is a first-class part of the walk: typing "gateway", "attention", or "observability" must still land
on the renamed section. An operator with the old vocabulary in their head is the realistic user here,
and losing them to a rename would be the actual regression.

Also re-read the restart/apply surfaces and the operator-verb error copy in this pass
(`lib/restart-presentation.ts`, `settings-apply-records-panel.tsx`): the humanized-error sweep
rewrote raw failure strings into plain sentences, so a settings failure should now name what did not
happen and what to do, without a Go error string as the primary text.
