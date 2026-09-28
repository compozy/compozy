---
id: RT-onboarding-skip-to-global
area: RT
title: First-run Skip starts in Global scope
persona: Lea
journey: J-19
expected: Workspaces is step 2 of 2. Continue is enabled with zero folders. The step heading stands alone; a HelpTip on it (`About workspace`) states that Skip starts in Global (~). A Skip control (`onboarding-skip-global`) reads "Skip" with no adjacent paragraph. Empty selection and the footer both report "None yet" without tutorial clauses. Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.
entry_points: web `/_app/` first-run; onboarding Workspaces step
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-workspaces.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-desktop.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-palette.png
last_report: docs/qa/reports/2026-08-26-pr-484-global-desktop.md
overlaps: RT-004; RT-onboarding-setup-panel-over-shell
---

Workspaces is step 2 of 2. Continue is enabled with zero folders. The step heading stands alone; a HelpTip on it (`About workspace`) states that Skip starts in Global (~). A Skip control (`onboarding-skip-global`) reads "Skip" with no adjacent paragraph. Empty selection and the footer both report "None yet" without tutorial clauses. Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.
