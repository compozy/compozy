---
id: ET-layout-editor-gaps-follow-canvas
area: ET
title: Edit window gaps at actual size in Settings
persona: Bruno
journey: J-administer-window-manager
expected: Settings › Layouts › Advanced › Spacing shows the Gaps card: a box drawn at actual size with four outer guides (Top gap, Right gap, Bottom gap, Left gap) and a cross in the middle (Gap between tiles); dragging an outer guide changes its gap one pixel per pixel of travel, the middle cross changes the gap between windows two pixels per pixel, and the readout (Between tiles, Top, Right, Bottom, Left) tracks it live within 0–64 px; each guide is a keyboard slider (arrows step 1, Shift+arrows step 10) with an announced value; the change waits for the save bar's "Save changes", after which split tabs in the shell use the new gaps; there is no snap-zone, corner, exit-slack, repeat-width or centre-binding control.
entry_points: Settings › Layouts › Advanced › Spacing › Gaps; Settings save bar
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager
---

story: As a person running agent work, I can see what a gap actually looks like before I save it.

qa-impact: 2026-07-24 new behavior, replacing eight pixel number fields. Both the canvas and the maps project into one declared 1440×900 reference screen, which is stated on the canvas. Flag only; the next QA cycle owns live testing.

QA impact 2026-09-28 (normie pass): "Spacing and snapping" moved into the Layouts page Advanced fold; open it before dragging guides. Flag only.

qa-impact: 2026-11-13 window-tabs-split S10. Snap bands, corner reach, exit slack, repeat
widths and top/bottom centre bindings were retired with tabs and splits, so the Advanced fold now
holds only "Spacing" (the Gaps card) and "History". The layout canvas above is the shell's split
view and does not draw gaps, so this scenario now checks the gap box itself and the live shell.
Walk: preconditions — a desktop with a split tab. 1) Open Settings › Layouts, expand Advanced and
find Spacing › Gaps. 2) Drag the top guide down about 10 px: "Top" in the readout rises by about 10
and the inner tiles move with it. 3) Drag the middle vertical guide 5 px to the right: "Between
tiles" rises by about 10. 4) Tab to the Left gap guide and press ArrowRight, then Shift+ArrowRight:
the value rises by 1 then by 10 and a screen reader announces it; it never passes 64 or goes below
0. 5) Confirm no control for snapping, corners, exit slack, repeat widths or centre bindings exists
on the page. 6) Press "Save changes" in the save bar, return to the desktop and confirm the split tab's outer
inset and the space between its windows changed. Reset for the current build.
