---
id: ET-palette-action-panel
area: ET
title: Run command and entity actions from the palette panel
persona: Bruno
journey: J-command-os-from-palette
expected: Command-K on the selected palette row opens a filterable action panel anchored to that row. Command rows expose their runnable action plus Pin or Unpin, Set alias, and Set shortcut; unavailable commands expose only those meta-actions and the daemon reason. Entity rows expose only real domain actions, destructive actions are unmistakable, action chords work from anywhere inside the palette without repeating, and a row removed by refresh closes the panel without firing against the vanished target.
entry_points: Command-K; command palette command rows; command palette entity rows
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-web-command-palette-shortcuts; ET-palette-personalization-lifecycle
---

Flagged by command-palette task 04. Task 12 owns the first real-user walk, visual-contract
comparison, and verdict.

2026-08-24 qa-impact (ENG-131): the concrete-row action now names the direct entity open (or the
Worktree scope action) rather than implying catalog search. Include one action-panel activation in
the direct-target walk.

Walk (task_11 plan):

1. Select any command row and press ⌘K — the panel opens anchored to the row with sections,
   per-action chord badges, and the primary action marked ↩; typing filters actions; ⌘K or Esc
   closes it.
2. Confirm Pin/Unpin, Set alias…, and Set shortcut… are present on every command row's panel;
   Set shortcut… deep-links into the settings shortcut table.
3. Open the panel on an entity row (a session) — only real domain actions render (land, interrupt,
   new tab here); destructive entries are visually unmistakable.
4. Open the panel on a disabled row — only meta-actions plus the daemon reason render; nothing
   unavailable is runnable.
5. Fire a panel action by its badge chord while list focus drifted — it executes once for the
   selected row; a held key does not repeat it.
6. Delete the selected entity from a second surface while its panel is open — the panel closes,
   selection falls to the nearest neighbor, nothing fires against the dead target.

Expected evidence: screenshots of the open panel (command row, entity row, disabled row), the
pin/alias effects visible on reopen, and the vanished-row closure; note the chord used for the
capture-phase dispatch check.
