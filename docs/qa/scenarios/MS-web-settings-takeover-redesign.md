---
id: MS-web-settings-takeover-redesign
area: MS
title: Settings takeover shell with srow pages and save models
persona: Dora
journey: J-administer-runtime-settings
expected: The settings window renders the 264px takeover sidebar (the host's Close Settings action closes the window; search with `/` shortcut filters sections; Basics/Personal/Agents/Advanced groups; runtime foot naming CompozyOS, never "daemon") collapsing to a chip strip under 56rem. Section labels read Remote access, Notifications, and Diagnostics, while their slugs stay `gateway`, `attention`, and `observability`, and searching the retired word still finds the renamed section. Pages use one-decision srows with consequence sentences, at most one Advanced fold per page, and choice cards with neutral selection. Draft pages show the floating save bar only when dirty/saving/error and flash "Saved" after a clean save; restart-needed changes surface the typed restart notice.
entry_points: web settings window (General, Memory, Automation, Skills, Hooks, Extensions, Diagnostics, Notifications, Remote access)
qa_status: untested
bug_ids: BUG-20261004-settings-search-shortcut-inactive
fix_status: fixed
retest_status: pass
fix_commits: b4166a6c2
evidence: docs/qa/evidence/2026-10-02-untested/settings-shortcut-final-dora-ended.json; docs/qa/reports/2026-10-02-untested.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-026; MS-037; ET-012; ET-044; ET-045
---

2026-08-20 retry: skipped by explicit user instruction. No settings search, save, or error path was walked.

Introduced by the opendesign settings redesign (docs/design/opendesign/_done/settings/settings-general.html and siblings, implemented 2026-07-21). Historical visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-S1/.

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

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-04 Dora navigation walk: `/` does not focus the visible search after fresh entry or a
neutral content click. The three retired-word aliases, browser Back and the compact chip strip
work. BUG-20261004-settings-search-shortcut-inactive owns the failure; complete save-model and
visual-reference legs remain pending. Evidence: settings-navigation-dora-ended.json and the linked
receipts under docs/qa/evidence/2026-10-02-untested/; all four screenshots inspected.

2026-10-04 repair: b4166a6c2 fixes keyboard entry and passes the original-persona final-bundle
replay. The fix/retest fields describe that defect; the full scenario remains untested while its
remaining save-model and visual-contract legs are open.

Reference reconciliation before the remaining visual walk: ff60d4ea1 moved the named General
prototype to `_done/settings/` with only the page title changing from Compozy to CompozyOS.
The current artifact retains that provenance; later font and retired-surface changes are recorded
in its Git history. The historical VC-S1 bundle is absent in both this worktree and the owner
checkout, so it supplies no current evidence. COPY.md's UI aliases own Basics/Personal/Agents/
Advanced; live OS host chrome owns Close Settings. These explicit owners replace the obsolete
group/exit wording above without changing search, navigation, save or restart requirements.
