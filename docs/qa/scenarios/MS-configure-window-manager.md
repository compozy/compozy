---
id: MS-configure-window-manager
area: MS
title: Configure window behavior and the project layout safely
persona: Bruno
journey: J-administer-window-manager
expected: Settings › Layouts shows, in order, Project layout (desktops → tabs → splits editor that reviews and applies inside its own card), Saved layouts, Window behavior (New windows: New tab or Split beside focus; Not enough room: One window at a time or Refuse the layout; Taking focus: Click and keys or Keys only; Switching desktops: Slide, Crossfade or Instant; Swap two windows modifier; Wrap directional focus; Focus follows the pointer), Shortcuts (Terminal preset, the shortcut table and global hotkeys, applied live) and an Advanced fold with Spacing (the Gaps card) and History (Layout history steps); there is no snap, corner, exit-slack, repeat-width, centre-binding, raise-on-focus, drag-away or group-move control and no floating option; out-of-range gaps or history limit and duplicate shortcut chords name the exact value at fault and block the save while the active known-good configuration stays; one save bar (Save changes / Discard) covers the global behavior while the layout applies from Apply layout and shortcuts apply at once; a valid save hot-applies to the next command without restarting; workspace layout overrides stay isolated; a config.toml carrying retired `[window_manager]` keys still loads (see MS-window-manager-retired-config-keys).
entry_points: Settings › Layouts; global config.toml; compozy config get|set|show|reload; GET/PATCH /api/settings/window-manager over HTTP and UDS; compozy layout-profile list|get|put|delete; compozy layout export|validate|apply
qa_status: untested
bug_ids: BUG-20260801-window-manager-live-config-drift
fix_status: fixed
retest_status: pass
fix_commits: d196f3a7
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/status-pending-restart.json; /Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/config-apply-history.json; /Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/nav-stack-limit.json; docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-window-manager-layout-recovery; ET-window-manager-layout-gestures
---

story: As a person running agent work, I can tune window behavior and layouts without accepting a partial or internally conflicting runtime configuration.

## Save recovery and application receipts

2026-09-10, issue #593: verify zero inner spacing, zero outer insets, and all-zero gaps independently,
then reload and inspect tiled layout geometry. Repeat a save, interrupt the settings request, and
verify the original draft remains retryable without another edit. Editing or discarding after a
failure must clear the old error. HTTP/application rejection must remain visible, including the
daemon's next action and warnings; HTTP 200 alone is not live-apply evidence. When persistence
succeeds but application fails, retry must reapply the pending layout rather than skip it as an
unchanged file, and Discard must retain the canonical saved baseline. Edit shortcuts while a
behavior draft is open and verify saving that draft preserves the latest shortcut maps and aliases.
For immediate shortcut, global-hotkey and alias edits, verify the saved section remains canonical
after application failure, no failure is announced as an applied success, and warnings plus
restart/new-session actions remain visible. Switch workspace during a pending edit and confirm
the result stays with its original scope.
The targeted run is tracked in `docs/qa/reports/2026-09-10-issue-593-layout-settings-save.md`.

qa-impact: 2026-07-22 replaced storage-limit settings with validated behavior defaults, shortcuts, bindings, gaps, snap thresholds, and declarative layout editing; 2026-07-24 added `window_manager.swap_modifier` (default `shift`) across config.toml, settings PATCH, Settings UI, and web gesture resolution; 2026-07-24 rebuilt Settings › Layouts as a direct-manipulation surface (canvas + inspector + docked review gate, diagram choice cards, gap box, snap map, repeat-width track, chord recorder, saved-layout cards) and added the `compozy layout-profile` CLI verbs. Flag only; the next QA cycle owns live retesting.

QA impact 2026-07-25 (deep-review remediation): ratio-track controls now keep stable semantic
identity and layout JSON export removes its temporary anchor after download. Flag only; the next QA
cycle owns direct-manipulation and import/export retesting.

QA impact 2026-07-26: layout-profile save/delete now mints and settles operation identities in the
store, preserves stale-version errors, and blocks delete confirmation dismissal while a write is
accepted. Status remains untested; no QA replay ran.

QA impact 2026-07-31: `nav_stack_limit`, `closed_entry_limit`, and tab shortcut actions joined the
live window-manager config contract. Reset for the window-tabs targeted cycle.

2026-08-16 qa-impact: shortcut values now accept arrays, indexed ranges, and explicit disables;
the daemon serves defaults and the effective map. Reset for the Herdr parity QA tail.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

A binding receipt that reports saved settings without confirmed application (`applied: false`,
`next_action: none`) must show its warnings with the retry error, including when no partial-failure
diagnostic is supplied.

QA impact 2026-09-28 (normie pass): gaps, snap zones, repeat widths and layout history now sit in the Layouts Advanced fold; "Import/Export JSON" read "Import/Export layout file"; the frame states which parts apply from Apply layout, Save, or immediately. Flag only.

Profile isolation regression: in a non-default profile, keep a desktop client attached and
report a failed global shortcut registration. Settings must show its failure reason and
confirmed active chord. Switch profiles and verify these statuses follow that profile's
client; settings ownership remains user/workspace scoped. HTTP reads and PATCH echoes
name the attachment with `client_id` and its profile with `client_profile`.

Enable an extension command only in a non-default profile. While that profile is selected,
change its shortcut and alias and reload: both changes must persist. A command enabled only
in the default profile must be refused when saved through the non-default client profile,
without changing the existing configuration.

Impact: saves validate the catalog selected by the existing `client_profile` selector.
HTTP/UDS and Web schemas, user/workspace config ownership, native tools, hooks, migrations,
workspace isolation, and official skill contracts are unchanged.

qa-impact: 2026-11-13 window-tabs-split S10. Floating windows, minimize, zoom, islands and
stacks were replaced by one tab bar per desktop whose tabs hold a window or a split. Settings lost
the snap map, the repeat-width track, centre bindings and the raise-on-focus, drag-away and
group-move controls; New windows offers only New tab and Split beside focus; the layout editor
edits desktops → tabs → splits and imports v4/v5/v6 files. Retired keys load with a warning and
have no control. Walk: preconditions — a project with two desktops and a split tab, and a second
project. 1) Open Settings › Layouts and read the sections listed in `expected`; open each
behavior group and the Advanced fold and confirm none of the retired controls appears. 2) Pick
Split beside focus under New windows, press Save changes, open a session from the shell and
confirm it joins the active tab as a split to the right of the focused window; switch back to New
tab, save, and confirm the next window opens as a new tab after the active one. 3) Run
`compozy config set window_manager.history_limit 0` and confirm it is refused with the field named
and `compozy config get window_manager.history_limit` is unchanged. 4) Record the same chord for two
actions in the shortcut table and confirm the second is blocked with the owner named. 5) Edit the
Project layout (move a seam), Review changes, Apply layout, and confirm the other project's layout
and `compozy config show` are unchanged. Reset for the current build.

QA impact 2026-11-13 (#1659): a project-scope shortcut edit stores only the project delta. With
`focus_policy` set in the user config, record one shortcut at project scope and confirm the project
`config.toml` gains `[window_manager.shortcuts]` and no scalar, gap, `global_shortcuts` or alias key;
then change `focus_policy` at user scope and confirm the project shows the new value. A project file
that already holds copied scalars loads the same values; the next project save removes the copies
equal to the inherited value and keeps the differing ones. Not walked yet; covered by
`TestUpdateSectionWindowManager` until the next window-manager QA cycle.
