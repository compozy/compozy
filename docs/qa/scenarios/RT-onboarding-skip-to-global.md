---
id: RT-onboarding-skip-to-global
area: RT
title: First-run Skip starts in Global scope
persona: Lea
journey: J-19
expected: Workspaces is step 2 of 2. Continue is enabled with zero folders. A Skip row (`onboarding-skip-global`) reads "Skip" with hint "Start in Global scope — your home folder, ~. Add project folders any time." Finishing without adding a folder lands on the live desktop: chip Global (`~`), Switch on and locked, no full-page workspace gate. The skip path does not `POST /api/workspaces/resolve` for `$HOME`. Adding a folder remains valid and turns Global off after selection.
entry_points: web `/_app/` first-run; onboarding Workspaces step
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-04
last_report:
overlaps: RT-004; RT-onboarding-setup-panel-over-shell
---

story: As a first-time adopter I can finish setup without pointing at a project folder and start working in Global scope.

Introduced 2026-08-12. The "Use global workspace" OptionCard and the zero-workspace full-page gate were deleted.

src: web/src/systems/onboarding/components/step-workspaces.tsx; web/src/systems/onboarding/hooks/use-onboarding-wizard.ts; web/src/systems/os/components/desktop-shell.tsx

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.
