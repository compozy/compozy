---
id: MS-web-settings-takeover-redesign
area: MS
title: Settings takeover shell with srow pages and save models
persona: Dora
journey: J-administer-runtime-settings
expected: The settings window renders the 264px takeover sidebar (the host's Close Settings action closes the window; search with `/` shortcut filters sections; Basics/Personal/Agents/Advanced groups; runtime foot naming CompozyOS, never "daemon") collapsing to a chip strip under 56rem. Section labels read Remote access, Notifications, and Diagnostics, while their slugs stay `gateway`, `attention`, and `observability`, and searching the retired word still finds the renamed section. Pages use one-decision srows with consequence sentences, at most one Advanced fold per page, and choice cards with neutral selection. Draft pages show the floating save bar only when dirty/saving/error and flash "Saved" after a clean save; restart-needed changes surface the typed restart notice.
entry_points: web settings window (General, Memory, Automation, Skills, Hooks, Extensions, Diagnostics, Notifications, Remote access)
qa_status: pass
bug_ids: BUG-20261004-settings-search-shortcut-inactive; BUG-20261004-settings-choices-ignore-window; BUG-20261004-settings-idle-timeout-display; BUG-20261004-settings-startup-false-offline; BUG-20261004-settings-offline-save-stuck
fix_status: fixed
retest_status: pass
fix_commits: b4166a6c2; baec8d019; 3268b7477; 4ce6fd811; 23dddb441
evidence: docs/qa/evidence/2026-10-02-untested/settings-idle-typeahead-dora-after-reload-observed.json; docs/qa/evidence/2026-10-02-untested/settings-idle-typeahead-dora-after-save.json; docs/qa/reports/2026-10-02-untested.md
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

2026-10-04 continuation: The clean 864px visual pair reveals permission choices that ignore the compact Settings layout. BUG-20261004-settings-choices-ignore-window owns the mismatch; use the existing window container token and re-walk. Save/error/restart coverage remains separate.

2026-10-04 final General replay: permission choices stack at 864px and retain three columns at
1440px. Both final visual-contract bundles pass after individual image inspection. Follow-up
behavior walks Unsaved, Discard, Saving and Saved; independent API reads and reload confirm both
the changed value and restoration of the complete baseline. The native timeout-select attempt
did not change its value and remains unverified. Other Settings pages and error/restart legs
remain pending; clean General parity and its successful save do not settle the full scenario.

2026-10-04 fresh Dora timeout walk: choosing 4 hours and saving persists 4h0m0s, but the
save bar remains dirty and reload shows Never. BUG-20261004-settings-idle-timeout-display owns
this divergence. The typed Restart needed notice is visible. Public PATCH cleanup restores the
entire baseline after the closed session. The compact-choice repair is verified at baec8d019;
other Settings-page/error/restart legs remain separate from this failed General save.

2026-10-04 repaired General replay: Dora saves four hours, observes Saving then Saved, and
reload still displays four hours with the typed restart notice. Restoring Never through Web
returns the complete baseline and current runtime state; invalid-duration public PATCH returns
400 without changing it. All five screenshots are inspected; the 20-frame recording is closed.
The existing 48 focused tests and Web typecheck/build pass; fix SHA and gate remain pending.

2026-10-04 delivery and startup finding: the idle-duration repair is verified at 3268b7477
after the exact-tree gate passes. Fresh entry also reveals an enabled Settings button before its
command is available; clicking it reports a reachable CompozyOS as unreachable and requires a
later click. BUG-20261004-settings-startup-false-offline owns that defect. The broad row remains
Fail until this repair and the remaining page/error legs are completed.


2026-10-04 remaining-page walk: startup availability is verified at 4ce6fd811. Memory and
Diagnostics save/reload/restore, negative retention validation, truthful empty Hooks and local-only
Remote access audit pass. Extensions persists, but an offline Save stays on Saving with both
actions disabled, then auto-submits on reconnect. BUG-20261004-settings-offline-save-stuck owns
that error-recovery failure. The closed 41-frame session, 15 inspected PNGs and independent
public reads are recorded in the report. All complete config baselines and current runtime
state are restored, with no restart or active sessions. The broad scenario remains Fail.


2026-10-04 final offline repair replay: Extensions now explains the failed save, keeps its draft
editable and permits Discard while offline. Reconnection alone does not write; explicit retry
shows Saved, persists through reload and can be restored. Memory and Notifications recovery
canaries pass. Eight final PNGs are inspected and the 26-frame recording is closed. Full config
baselines and current runtime state are restored. The broader page/navigation/visual legs have
current-cycle proof; required gate and commit bookkeeping remain before promotion. Evidence:
settings-offline-guidance-dora-ended.json and related receipts in the report evidence directory.

2026-10-04 closure: offline recovery is verified at 23dddb441 after the exact-tree gate passes
(693 Web files / 6,982 tests). All named page, navigation, visual, draft/save/error and typed
restart legs have current-cycle proof. Set the complete scenario to pass/fixed; all five
linked defects are verified. The separate Notifications delivery charter retains its own gaps.
