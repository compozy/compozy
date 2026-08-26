---
id: RT-onboarding-skip-to-global
area: RT
title: First-run Skip starts in Global scope
persona: Lea
journey: J-19
expected: Workspaces is step 2 of 2. Continue is enabled with zero folders. The step heading stands alone; a HelpTip on it (`About workspace`) states that Skip starts in Global (~) and that setup does not enable Network. A Skip control (`onboarding-skip-global`) reads "Skip" with no adjacent paragraph. Empty selection and the footer both report "None yet" without tutorial clauses. Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.
entry_points: web `/_app/` first-run; onboarding Workspaces step
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-workspaces.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-desktop.png; docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-onboarding-global-skip-palette.png
last_report: docs/qa/reports/2026-08-26-pr-484-global-desktop.md
overlaps: RT-004; RT-onboarding-setup-panel-over-shell
---

story: As a first-time adopter I can finish setup without pointing at a project folder and start working in Global scope.

Introduced 2026-08-12. The "Use global workspace" OptionCard and the zero-workspace full-page gate were deleted.

src: web/src/systems/onboarding/components/step-workspaces.tsx; web/src/systems/onboarding/hooks/use-onboarding-wizard.ts; web/src/systems/os/components/desktop-shell.tsx

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: Lea selected the native Codex runtime, reached Workspaces, added and removed a project through the directory browser, then used Skip with zero project folders. The desktop opened in locked Global scope, the public workspace read contained only the operator-home registration, and refresh preserved the first-run completion state.

2026-08-20 qa-impact: workspace-step copy density. The step subtitle, Skip paragraph, Network paragraph, and footer tutorial clause were removed; Skip/Global/Network consequences moved into the heading HelpTip. Reset for a copy walk.

2026-08-23 qa-impact (Profiles): the Skip path still lands in Global, but Global now means the
across-workspaces view rather than a pseudo-workspace, and the home directory is refused at the
daemon rather than merely not resolved. Already `untested`, so no reset was needed. Confirm the
first-run desktop is quiet about profiles when only `default` exists, and that finishing without a
folder produces a usable Global view. What Global means for the data is owned by
`MS-global-scope-no-workspace-work`.

2026-08-26 re-walk: Lea selected Codex through the public first-run flow, added and removed one
project, then chose Skip with no project folders. Global opened with a usable Dock and command
palette; a refresh preserved Global and the palette catalog. Independent CLI and HTTP reads showed
an empty workspace catalog and a healthy `workspace=global` command catalog.
