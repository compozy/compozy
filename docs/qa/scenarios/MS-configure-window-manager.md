---
id: MS-configure-window-manager
area: MS
title: Configure window behavior and declarative layouts safely
persona: Bruno
journey: J-administer-window-manager
expected: Settings › Layouts exposes every supported `[window_manager]` value through direct manipulation — a desktop canvas, a 1:1 gap box, a snap-zone map, a repeat-width track, and a chord recorder — with no number field bound to a geometry value; out-of-range gaps, snap thresholds, history limit, duplicate repeat widths, and duplicate shortcut chords each name the exact value at fault and block the save while preserving the active known-good configuration; one floating save bar covers the global config and the workspace layout reviews inside its own card; a valid save hot-applies to the next command without restarting; workspace layout overrides remain isolated.
entry_points: Settings › Layouts; global config.toml; compozy config get|set|apply; GET/PATCH /api/settings/window-manager over HTTP and UDS; compozy layout-profile list|get|put|delete
qa_status: pass
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
