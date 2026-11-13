---
id: ET-layout-editor-shortcut-recorder
area: ET
title: Record, reset and de-conflict window-manager shortcuts, including retired bindings
persona: Bruno
journey: J-administer-window-manager
expected: Clicking a chord records the next real key combination; a bare key or a modifier-only press is refused with a spoken reason; recording an action's shipped default clears the override instead of storing a copy; only overrides are persisted; two overrides on one chord block the save and say so, while an override landing on another action's shipped chord is stored and explained as shadowing; Escape cancels recording; reset returns one action or all of them to the shipped keymap; the table lists "Move to new tab", "Move tab left" and "Move tab right" with no chord (they ship without one, so no "unbound" flag) and lets them be recorded, and window.tile.left|right|top|bottom as "Move to tab's left edge", "Move to tab's right edge", "Move to tab's top edge" and "Move to tab's bottom edge"; a stored binding to a retired action shows as its own row under its old name with the "retired" flag and the hint "This action was retired with tabs and splits, so the shortcut does nothing. Remove it.", its chord button and add-alternate button disabled, and its reset button ("Remove the retired shortcut for …") deleting the binding; a retired binding never blocks another action's chord.
entry_points: Settings › Layouts › Shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa; docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: MS-configure-window-manager
---

story: As a person running agent work, I can change a window shortcut and be told immediately if the daemon will not accept the result.

qa-impact: 2026-07-24 new behavior, replacing a raw JSON textarea. The two conflict classes are distinguished because `CanonicalShortcuts` rejects duplicate overrides but accepts an override that shadows a default. Flag only; the next QA cycle owns live testing.

2026-08-16 qa-impact: the recorder now manages ordered alternates and expanded range families;
conflict diagnostics run against the complete effective keymap. Reset for the Herdr parity QA tail.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

qa-impact: 2026-11-13 window-tabs-split S10. Minimize, zoom, float, the four corner tiles,
merge all and detach tab were retired; their stored bindings still load, do nothing and appear as
retired rows. The four edge tiles were retargeted to "Move to tab's … edge", and window.unsplit,
window.tab.move_left and window.tab.move_right were added unbound. Walk: preconditions — in the
user config.toml set `[window_manager.shortcuts]` `"window.zoom" = "control+alt+KeyZ"` and
`"window.tile.top-left" = "control+alt+shift+KeyU"`, then run `compozy config reload`. 1) Settings ›
Layouts › Shortcuts: rows "Zoom window" (`window.zoom`) and "Tile to top left"
(`window.tile.top-left`) show their chords, the "retired" flag and the hint; clicking the chord
does nothing (disabled). 2) Record Control+Alt+Z on "Move to new tab": it is accepted without a
conflict notice, because a retired binding owns no chord. 3) Press the reset button on "Zoom
window": the row disappears and `compozy config get window_manager.shortcuts -o json` no longer
lists `window.zoom`. 4) Confirm "Move tab left" and "Move tab right" have no chord, no "unbound" flag, and can be
recorded, and that the window.tile.left row reads "Move to tab's left edge" with its default chord (`control+alt+ArrowLeft`).
5) Repeat the original checks: bare key refused, Escape cancels, duplicate override blocked,
shadowing explained, reset all. Reset for the current build.
