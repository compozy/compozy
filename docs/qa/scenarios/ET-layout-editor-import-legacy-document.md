---
id: ET-layout-editor-import-legacy-document
area: ET
title: Import a layout file saved before tabs into the Settings layout editor
persona: Bruno
journey: J-administer-window-manager
expected: Import layout file on Settings › Layouts › Project layout accepts a version 4, 5 or 6 JSON document; a version 4 or 5 file is sent to the daemon's `layout.replace` preview and the editor opens the version 6 result it returns — tiled groups that can be cut apart become one split tab, every floating, minimized and non-active stacked window becomes a single-window tab after the tiled tabs, a pinned single window stays pinned, and the zoomed window's tab is the one the desktop opens on ("Opens here"); nothing is applied until Review changes and Apply layout; a file that is empty, not JSON or not an object shows "The selected file is not a layout document."; a file the browser cannot read shows "The selected file could not be read."; a document the daemon refuses (for example a version it does not accept) shows "CompozyOS can't open this layout document. It may come from a newer version." and a check that does not finish shows "Couldn't check the layout file. Try again.", never a parser, transport or daemon error code, and the draft stays as it was; Export layout file then downloads `layout-<workspace>.json` with `"version": 6`.
entry_points: Settings › Layouts › Project layout › Import layout file, Export layout file; review bar (Review changes, Apply layout, Discard)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-layout-editor-load-saved-layout; MS-layout-profile-cli-roundtrip; MS-configure-window-manager
---

story: As an operator with layout files exported before tabs and splits, I can still open them in Settings and get tabs I can review before anything changes.

scope: US-022 AC-1, AC-2, EC-1, EC-2 of spec `window-tabs-split`. The web has no converter; the conversion is the daemon's v5 → v6 migration (spec "Migration v5 → v6").

qa-impact: 2026-11-13 window-tabs-split S10. New scenario. Walk: preconditions — a project open in
the shell, and two files exported with `compozy layout export -o json` from earlier builds: a version 5
document (v0.6) whose desktop has two side-by-side tiled groups, one of them a three-window stack,
plus one pinned floating window and one minimized window; and a version 4 document; plus a text file renamed `bad.json`. 1) Settings › Layouts › Project layout › Import
layout file, choose the v5 file: the Desktops strip shows its desktops; the Tabs strip shows the
merged split tab (holding the stack's active member), then one tab per other stack member, then the minimized window as a single-window tab, while
the pinned floating window's tab sits first with the pin mark; the review status
reads "Unreviewed edits · CompozyOS checks them before they apply" and the shell is unchanged. 2)
Click a tab with a split and confirm the canvas draws it and the inspector shows Split as and
Balance. 3) Press Review changes: "Checked · changes …" counts desktops, tabs, splits and windows, singular for one ("1 split") and left out at zero, never "branch"; press Apply layout: the shell shows the
same tabs. 4) Import the v4 file: the editor shows tabs only, as in step 1; press Discard. 5) Import
`bad.json`: "The selected file is not a layout document." appears in the review bar and the draft is
unchanged. 6) Press Export layout file and open the downloaded file: `"version": 6`, desktops carry
`tabs` with `root` nodes, and there is no `groups`, `floating` or `minimized` field.
