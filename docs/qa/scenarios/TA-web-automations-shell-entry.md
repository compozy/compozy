---
id: TA-web-automations-shell-entry
area: TA
title: Dock, palette, Settings and Loop pages reach Automations
persona: Cora
journey: J-09
expected: "The dock's group 2 reads Tasks · Loops · Automations with one bolt launcher (tooltip Automations, open dot while a window is open) and no Jobs or Triggers launcher. The command palette has one Automations view, an Automations entity section that finds both kinds by name and sentence and opens `/automations/jobs/{id}` or `/automations/triggers/{id}`, and the actions New scheduled automation, New automation on an event and Open Automations. Settings › Automation shows one Manage row Automations (\"N automations, M on · X scheduled, Y on events\") that opens the window, with the plainer labels (Scheduled automations use this time zone; Scheduled automations at once; Default run limit; the Run automation unavailable alert) and saves the same `config.toml` keys. A Loop page replaces Add trigger / Add schedule with one ghost Automate ▾ menu (On a schedule · When something happens) that opens the editor with Start this Loop locked to that Loop, and its Start panel Automations row reads Manual only or N automations linking to `/automations?loop=<name>`."
entry_points: web dock; command palette (⌘K); web Settings › Automation; web Loop detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-domain-views; TA-web-automation-editor; LP-033
---

New in the Automations spec (task 07; US-028–US-031). Covers the shell registrations that replaced the Jobs and Triggers apps. Visual contract: `docs/design/opendesign/automations/automations-surfaces.html` (surfaces VC-01…06). Settings label changes must not change any persisted key; verify with `compozy config get automation -o json` before and after saving.
