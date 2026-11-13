---
id: ET-layout-editor-split-weights
area: ET
title: Splits built in the Settings layout editor pass daemon validation
persona: Bruno
journey: J-administer-window-manager
expected: Every split the editor produces — by "Bring a window beside it", by changing Rows/Columns, by dragging a seam, by "Distribute evenly", by "Move to new tab" or "Remove from layout" on a split member — keeps positive weights that sum to 1, so Review changes never returns `topology.split_weight_sum`, `topology.split_weight_count` or `topology.split_not_flattened`; the Balance list shows each member's share as a named fraction or a percentage.
entry_points: Settings › Layouts › Project layout; selection inspector › Bring a window beside it, Split as, Balance, Distribute evenly
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager; ET-layout-editor-split-orientation
---

story: As a person running agent work, a layout I build in Settings is one the daemon will accept.

qa-impact: 2026-07-24 fixes a defect: every split created in Settings emitted a weight of 1 per child, and the daemon requires the vector to sum to 1 with no normalization pass, so "Validate and preview" always failed. Flag only; the next QA cycle owns live testing.

qa-impact: 2026-11-13 window-tabs-split S10. The editor edits tabs and splits; the Stack
conversion and group branches are gone, and windows join a split through "Bring a window beside it"
(Left, Right, Above, Below) or leave it through "Move to new tab". Walk: preconditions — a desktop
with at least four single-window tabs. Open Settings › Layouts, pick the first tab and click its
window. 1) Under "Bring a window beside it" keep Right and pick a window from another tab: the canvas
shows a two-member split and that window's tab leaves the Tabs strip. 2) Repeat with Below on the new
member, then with Right again, so the tab holds a nested split. 3) Drag a seam to an uneven share,
then press Distribute evenly: the Balance list reads ½ · ½ (or ⅓ each for three). 4) Select a member
and press Move to new tab: a new tab appears in the strip and the remaining split's shares still add
up to the whole. 5) Press Remove from layout on another member. 6) After each step press Review
changes: the status reads "Checked · …" and the red "CompozyOS refused this layout" list never
appears. Reset for the current build.
