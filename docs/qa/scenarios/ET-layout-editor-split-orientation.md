---
id: ET-layout-editor-split-orientation
area: ET
title: Rows and Columns produce the arrangement they show
persona: Bruno
journey: J-administer-window-manager
expected: Choosing Rows in the inspector stacks the windows top to bottom and choosing Columns places them side by side, matching the diagram on each button and the arrangement the live shell renders for the same document; the inspector never shows an axis name.
entry_points: Settings › Layouts; selection inspector › Arrange as
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; ET-layout-editor-split-weights
---

story: As an operator, the button I press produces the shape it draws.

qa-impact: 2026-07-24 fixes a defect: "Split rows" mapped to the axis that divides width, so it produced columns. The axis vocabulary left the UI entirely and the mapping now has one owner with a regression asserted through the runtime projector. Flag only; the next QA cycle owns live testing.
