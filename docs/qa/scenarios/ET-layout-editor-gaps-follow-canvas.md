---
id: ET-layout-editor-gaps-follow-canvas
area: ET
title: Editing gaps and snap zones updates the layout canvas at real scale
persona: Bruno
journey: J-administer-window-manager
expected: Dragging a gap guide moves one pixel of gap per pixel of travel, the readout tracks it live, and the layout canvas above re-projects with the new gaps; snap bands, corner reach and exit slack redraw as the region they govern; each guide is also an arrow-key slider with an announced value; centre-zone bindings open a real menu and only offer the values the daemon accepts.
entry_points: Settings › Layouts › Spacing and snapping
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager
---

story: As an operator, I can see what a gap or a snap band actually looks like before I save it.

qa-impact: 2026-07-24 new behavior, replacing eight pixel number fields. Both the canvas and the maps project into one declared 1440×900 reference screen, which is stated on the canvas. Flag only; the next QA cycle owns live testing.
