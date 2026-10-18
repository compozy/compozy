---
id: APP-desktop-native-edit-shortcuts
area: APP
title: Copy and paste through the desktop native Edit menu
persona: Dora
journey: J-desktop-attach-daily
expected: The packaged desktop app exposes the platform Edit menu and Command or Control cut, copy, paste, and select-all actions work in editable product fields without changing page zoom shortcuts.
entry_points: packaged desktop main window keyboard shortcuts; native Edit menu
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: APP-desktop-page-zoom
---

The walk must copy selected text from one editable renderer field, paste it into another through the
platform shortcut, and confirm the Edit menu contains the native editing actions.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


QA impact 2026-10-10 (verification pending): Verify right-click Copy on selected transcript text and Cut/Paste/Select All in the composer. Open a misspelled word menu, choose a suggestion, add a word to the dictionary, and toggle spelling off. On Windows/Linux select English and Portuguese together; on macOS confirm the system-language explanation. Relaunch and confirm spelling preferences survive. Confirm tab, dock, and loop-editor custom menus remain usable. Owning automated suites: desktop context-menu Vitest and packaged shell E2E.

Verification 2026-10-10: The packaged macOS Electron suite passed 24 unchanged cases, including native Edit shortcuts. The focused context-menu case passed after correcting the test driver to dispatch macOS roles through AppKit and wait for window focus. It verified native menu availability, real clipboard contents, disabled spelling after relaunch, and the renderer dock, tab, and loop-editor node menus. The cold-start deep-link case passed on repeat after an initial app-exit timeout. Dictionary suggestions and multilingual controls are covered at the Electron boundary by the desktop unit suite; the packaged test also selects and restores English and Portuguese on Windows/Linux; those native results and the remaining manual transcript/composer spelling walks remain pending.

Automated retest 2026-10-10: [Linux packaged desktop CI](https://github.com/compozy/compozy/actions/runs/38111594503/job/114388256118) passed all 26 cases at `0fd722591`. The native context-menu case exercised real clipboard Copy, simultaneous English/Portuguese selection, disabled spelling and both languages after relaunch, and dock/tab/loop-editor custom menus. This closes the pending Linux automated result above; it does not claim a full manual transcript/composer dictionary walk.
