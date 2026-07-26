---
id: ET-layout-editor-split-weights
area: ET
title: A split created in Settings passes daemon validation
persona: Bruno
journey: J-administer-window-manager
expected: Converting a branch to Rows, Columns or Stack — at any number of windows — produces a document the daemon validates without `topology.split_weight_sum`; Distribute evenly produces the same; the balance readout shows each pane's share as a fraction.
entry_points: Settings › Layouts; selection inspector › Arrange as
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; ET-layout-editor-split-orientation
---

story: As an operator, a layout I build in Settings is one the daemon will accept.

qa-impact: 2026-07-24 fixes a defect: every split created in Settings emitted a weight of 1 per child, and the daemon requires the vector to sum to 1 with no normalization pass, so "Validate and preview" always failed. Flag only; the next QA cycle owns live testing.
