---
id: ET-layout-editor-load-saved-layout
area: ET
title: Load a saved layout into the editor, including one saved before tabs, without losing edits
persona: Bruno
journey: J-administer-window-manager
expected: Under Settings › Layouts › Saved layouts each card shows the default tab of its first desktop as a thumbnail with "This project" or "Every project", the tab and window counts, Load and a delete button; a card for a layout still stored in a version 4 or 5 document shows "Saved before tabs" and "Opens as tabs when loaded" instead of a thumbnail; Load on a clean editor replaces the draft at once, and on a draft with unapplied edits opens "Replace unapplied layout edits?" (Keep editing leaves the draft untouched, Replace edits loads); loading a pre-tabs layout sends it to the daemon's `layout.replace` preview and opens the version 6 tabs it returns, or shows "This saved layout could not be opened."; the loaded draft still needs Review changes and Apply layout; delete asks "Delete this saved layout?" and leaves the editor untouched.
entry_points: Settings › Layouts › Saved layouts (Load, delete, Save the current layout); compozy layout-profile put|list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager
---

story: As a person running agent work, loading or deleting a saved layout never silently destroys work I have not applied yet.

qa-impact: 2026-07-24 fixes a defect: selecting a profile replaced the whole draft with no confirmation and no dirty check, and a delete left three of the removed record's fields seeded into the next one. Flag only; the next QA cycle owns live testing.

QA impact 2026-07-25 (deep-review remediation): saved-layout mutations now use the profile's scope
identity and current expected version, and cache updates target the exact workspace. Flag only; the
next QA cycle owns save, load, overwrite, and delete retesting across workspaces.

qa-impact: 2026-11-13 window-tabs-split S10. Saved layouts now hold version 6 documents
(desktops → tabs → splits); cards show tab and window counts; a pre-tabs (v4/v5) record is shown
without a guessed thumbnail and is converted by the daemon on Load, through the same
`layout.replace` preview the import uses, with no converter in the web. The daemon also rewrites
stored v4/v5 profiles to v6 when it boots, so after a restart such a record normally shows a
thumbnail. Walk: preconditions — a project with a split tab, and a saved v5 layout spec file (an
export from a v0.6 build, wrapped in a `window_layout` spec with `version: 1`, `id`,
`display_name`, `aspect_variant` and `overflow_policy`). 1) Settings › Layouts › Saved layouts:
press "Save the current layout", fill Name and Resource ID, press Save layout; a card appears with
the current tab's shape and its tab and window counts. 2) Without editing, press Load on it: the
draft is replaced and no dialog opens. 3) Drag a seam on the canvas, then press Load: "Replace
unapplied layout edits?" opens; Keep editing keeps the moved seam; Load again and Replace edits
restores the saved shares. 4) Run `compozy layout-profile put pre-tabs --file <v5-spec> --expected-version 0 -o json`
and reload Settings: the card either shows "Saved before tabs" / "Opens as tabs when loaded" or,
when the daemon already stored it as v6, a tab thumbnail; Load it either way and confirm the
editor shows tabs only (floating, minimized and stacked windows of the source each became a tab of
their own) and the review status reads "Unreviewed edits · CompozyOS checks them before they
apply". 5) Press Review changes then Apply layout and confirm the shell shows the same tabs. 6)
Press the card's delete button: "Delete this saved layout?" opens; Delete layout removes the card
and the editor keeps its draft. Reset for the current build.
