---
id: ET-layout-editor-group-overlap-refused
area: ET
title: Dragging a group edge into a neighbour is flagged before it can apply
persona: Bruno
journey: J-administer-window-manager
expected: A group edge dragged past a neighbour marks both groups on the canvas immediately, the inspector says the daemon refuses overlapping groups, Review returns `topology.group_overlap` with its path, Apply stays disabled, and no preview is requested for the refused document.
entry_points: Settings › Layouts; layout canvas group edge
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager
---

story: As an operator, I find out that a layout is impossible while I am building it, not after I try to apply it.

qa-impact: 2026-07-24 new behavior. The canvas evaluates group overlap client-side for the immediate mark and still routes the decision through the daemon's validate step. Flag only; the next QA cycle owns live testing.
