---
id: MS-window-manager-retired-config-keys
area: MS
title: Retired window-manager config keys still load with a warning
persona: Bruno
journey: J-administer-window-manager
expected: A config.toml whose `[window_manager]` table carries raise_on_focus, drag_away_policy, group_move_modifier, snap.edge_band, snap.corner_reach, snap.exit_slack, snap.repeat_ratios, bindings.top_center and bindings.bottom_center — with any value, including values that used to be invalid — loads, and `compozy config show` (also get, list and validate) prints `warning: window_manager.<key> is retired and ignored; remove it before v0.8.0.` on stderr for each key; `new_window_policy = "floating"` loads as `tab` with `warning: window_manager.new_window_policy = "floating" is deprecated; use "tab" before v0.8.0.`; the effective config and the settings API omit every retired key and report `new_window_policy` as `tab`; Settings › Layouts has no control for them and saving the page does not write them back; a layout document whose `overrides` carry the same keys validates and applies, the daemon log recording the `window_manager.deprecated_surface_used` warning; the daemon keeps running and new windows open as tabs.
entry_points: user config.toml [window_manager]; compozy config show|get|list|validate|reload; GET /api/settings/window-manager; Settings › Layouts; compozy layout validate|apply (document overrides); daemon log
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; MS-layout-profile-cli-roundtrip
---

story: As an operator upgrading from a build with floating windows and snapping, my config keeps loading and tells me exactly which keys to remove before they stop being accepted.

scope: US-019 AC-1, AC-2, EC-1, EC-2 of spec `window-tabs-split` (Config Lifecycle). Retired keys are accepted until v0.8.0; removal is a later, separate change.

qa-impact: 2026-11-13 window-tabs-split S10. New scenario. Walk: preconditions — an isolated lab
home with the daemon running and one project open. 1) Add to the user config.toml:

```toml
[window_manager]
raise_on_focus = true
drag_away_policy = "not-a-policy"
group_move_modifier = "alt"
new_window_policy = "floating"

[window_manager.snap]
edge_band = -4
corner_reach = 24
exit_slack = 12
repeat_ratios = [0.5, 0.5]

[window_manager.bindings]
top_center = "maximize"
bottom_center = "nothing"
```

2) Run `compozy config show 2>warn.txt`: it succeeds, and warn.txt holds one "is retired and
ignored; remove it before v0.8.0." line for each of the nine keys plus the "floating" deprecation
line; the printed config has no raise_on_focus, drag_away_policy, group_move_modifier, snap or
bindings entries and shows `new_window_policy = "tab"`. Note whether any key warns more than once
(the spec asks for once per key). 3) Run `compozy config validate` and `compozy config reload`:
both succeed. 4) Open Settings › Layouts: New windows shows New tab selected; no control exists
for any retired key. Change "Taking focus", press Save changes, and confirm config.toml gained no
new retired key. 5) Open a session from the shell: it opens as a new tab. 6) Take
`compozy layout export -o json`, add `"overrides": {"raise_on_focus": true, "snap": {"edge_band": 9},
"new_window_policy": "floating"}`, run `compozy layout validate --file` and
`compozy layout apply --file --revision <current>`: both succeed, `compozy layout export` shows the
overrides without the retired keys and with `new_window_policy` `tab`, and the daemon log has a
`window_manager.deprecated_surface_used` warning naming each key. 7) Remove the keys and run
`compozy config show` again: no warning.
