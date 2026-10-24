---
id: TA-web-automations-shell-entry
area: TA
title: Dock, palette, Settings and Loop pages reach Automations
persona: Cora
journey: J-09
expected: "The dock's group 2 reads Tasks · Loops · Automations with one bolt launcher (tooltip Automations, open dot while a window is open) and no Jobs or Triggers launcher. The command palette has one Automations view, an Automations entity section that finds both kinds by name and sentence and opens `/automations/jobs/{id}` or `/automations/triggers/{id}`, and the actions New scheduled automation, New automation on an event and Open Automations. Settings › Automation shows one Manage row Automations (\"N automations, M on · X scheduled, Y on events\") that opens the window, with the plainer labels (Scheduled automations use this time zone; Scheduled automations at once; Default run limit; the Run automation unavailable alert) and saves the same `config.toml` keys. A Loop page replaces Add trigger / Add schedule with one ghost Automate ▾ menu (On a schedule · When something happens) that offers only the starts the Loop allows and opens the editor with Start a Loop locked to that Loop, and its Start panel shows N automations linking to `/automations?loop=<name>` (Manual only as the gist when none; \"This Loop can only be started by hand.\" when it allows no automation start)."
entry_points: web dock; command palette (⌘K); web Settings › Automation; web Loop detail
qa_status: pass
bug_ids: Q1-F4 fixed
fix_status: fixed
retest_status: pass
fix_commits: 99910ca2b
evidence: .compozy/tasks/automations/screens/; .compozy/tasks/automations/reports/q1-qa-walk.md (Round 2)
last_report: docs/qa/reports/2026-10-22-issue-888-automation-timezone.md
overlaps: ET-palette-domain-views; TA-web-automation-editor; LP-033
---

New in the Automations spec (task 07; US-028–US-031). Covers the shell registrations that replaced the Jobs and Triggers apps. Visual contract: `docs/design/opendesign/automations/automations-surfaces.html` (surfaces VC-01…06). Settings label changes must not change any persisted key; verify with `compozy config get automation -o json` before and after saving.

## Schedule time zone regression

- In a fresh isolated home with `TZ=America/Sao_Paulo`, create the first user config and confirm
  `automation.timezone` is explicitly persisted as `America/Sao_Paulo`. A daily `30 8 * * *`
  schedule must resolve to 08:30 in that zone.
- Repeat with an invalid host zone and confirm an explicit `UTC` fallback without startup failure.
- On a host whose `/etc/localtime` links to an IANA zoneinfo file, repeat with `TZ=/etc/localtime`
  and `TZ=:/etc/localtime`: both must persist that zone. A nonexistent absolute zoneinfo path
  must fall back to UTC rather than infer a zone from its filename.
- Open an existing config without the key, including an empty file, and repeat installation:
  its effective zone must remain UTC. Preserve an explicit pre-existing zone as well.
- In Settings → Automation, search for `Tokyo`, select `Asia/Tokyo`, save, and reopen. Verify the
  config read and automation sentence/preview use the saved zone.
- Search the full IANA name `Etc/GMT+3`, which browsers omit from their standard zone list;
  select, save, and reload it. Invalid names and bare numeric offsets must not change the selection.
- With a configured zone different from the daemon host, verify the inline mismatch notice and
  use the machine-zone shortcut. Save and confirm the notice disappears. The daemon host zone,
  not the browser zone, owns this shortcut over remote connections.
- Check keyboard search/selection, no-match search without changing the saved value, and a narrow
  viewport. When host detection fails, the picker remains usable without a machine-zone shortcut.
- Submit an invalid zone through the settings API and confirm validation rejects it without
  persisting or applying it. Confirm CLI `config set automation.timezone` still works.
- With a registered daily 08:30 cron, change UTC to America/Sao_Paulo and restart the isolated
  daemon. Verify next run moves from 08:30Z to 11:30Z. Restart again without changing the zone:
  next run must stay identical. Existing legacy cursors and catch-up work survive upgrade, and
  deferred reserved fires complete once before the new-zone cursor continues (owning SQLite
  scheduler integration suite). Interval and absolute timestamp schedules do not move.
