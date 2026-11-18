---
id: ET-layout-editor-split-orientation
area: ET
title: Rows and Columns in the layout inspector produce the split they draw
persona: Bruno
journey: J-administer-window-manager
expected: With a window of a split selected on the Settings layout canvas, the inspector's "Split as" section offers Rows and Columns, each drawn as the arrangement it makes; Rows stacks the split's windows top to bottom and Columns places them side by side, matching the live shell after Apply layout; choosing the direction of an enclosing split merges the two into one split instead of nesting the same direction; the inspector never names an axis; a single-window tab shows no "Split as" section.
entry_points: Settings › Layouts › Project layout; selection inspector › Split as
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager; ET-layout-editor-split-weights
---

story: As a person running agent work, the button I press produces the shape it draws.

qa-impact: 2026-07-24 fixes a defect: "Split rows" mapped to the axis that divides width, so it produced columns. The axis vocabulary left the UI entirely and the mapping now has one owner with a regression asserted through the runtime projector. Flag only; the next QA cycle owns live testing.

qa-impact: 2026-11-13 window-tabs-split S10. The inspector section is now "Split as" (was
"Arrange as") and only splits exist: the Stack option and group branches are gone, and a split lives
inside one tab. Walk: preconditions — a desktop with a tab of two windows side by side and a tab with
a single window. Open Settings › Layouts and pick the split tab. 1) Click one window on the canvas:
the inspector shows Swap with, Bring a window beside it, Split as and Balance; Columns is pressed. 2)
Press Rows: the canvas stacks the two windows top to bottom and the Balance labels read "Row 1",
"Row 2". 3) Press Columns: they return side by side ("Column 1", "Column 2"). 4) With Rows active,
use "Bring a window beside it" with Right on a window from another tab, then select the new window
and press Rows: the canvas shows one three-row split, not a split nested in a split. 5) Pick the
single-window tab and click its window: no "Split as" section. 6) Review changes, Apply layout, and
confirm the live tab has the arrangement the canvas showed. Reset for the current build.
