---
id: RT-onboarding-skip-to-global
area: RT
title: First-run Skip starts in Global scope
persona: Lea
journey: J-19
expected: Project is step 2 of 2 ("Choose a project folder"). Continue is enabled with zero folders. A visible line under the heading (`onboarding-workspace-help`) reads "Agents can read and change files in the folders you add. You can add more later." A Skip control (`onboarding-skip-global`) reads "Skip — use my home folder". Empty selection shows "None yet" with "Folders you add appear here.", and the footer reports "None yet" under a "Projects" label. Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.
entry_points: web `/_app/` first-run; onboarding Workspaces step
qa_status: pass
bug_ids: BUG-20261002-onboarding-long-path-clipping
fix_status: fixed
retest_status: pass
fix_commits: 951105e0f
evidence: docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-workspaces.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-desktop.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-palette.png; docs/qa/evidence/2026-10-02-untested/onboarding-long-path-before.png; docs/qa/evidence/2026-10-02-untested/onboarding-long-path-fixed-desktop.png; docs/qa/evidence/2026-10-02-untested/onboarding-fixed-layout-widths.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-004; RT-onboarding-setup-panel-over-shell
---

Project is step 2 of 2. Continue is enabled with zero folders. The step help is a visible line under the heading, and the Skip control (`onboarding-skip-global`) reads "Skip — use my home folder". Empty selection and the footer both report "None yet". Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA impact 2026-09-28 (ui-normie-pass): step labels read Model / Project, the step help is visible text
instead of a HelpTip, Back hides on step 1, the per-row "add folder" button stays visible at reduced
emphasis, and the model facts line shows only the price ("$3 in / $15 out per million tokens"). Not walked.

2026-10-02: Live setup, add/remove gating, offline resume, daemon-unavailable commit errors, and recovery were walked. Long-path clipping was reproduced and repaired; fresh browser replay passes at four viewport widths. Final audit and gate remain pending.

2026-10-02 completed replay: Fresh browser reload and independent API reads confirmed completed setup in Global with zero workspaces/sessions; strict lab audit passed and teardown was clean. Fix 951105e0f; affected local gate passed. See the current dated report for exact receipts and limitations.
