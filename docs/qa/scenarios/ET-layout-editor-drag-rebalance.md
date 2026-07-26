---
id: ET-layout-editor-drag-rebalance
area: ET
title: Rebalance a split by dragging its divider in Settings
persona: Bruno
journey: J-administer-window-manager
expected: Dragging a divider on the Settings canvas moves it under the pointer at reference scale, snaps to a half, third or quarter with a live readout, and keeps the split's weights summing to 1 at every frame; arrow keys move the same divider without a pointer; the daemon accepts the result and never returns `topology.split_weight_sum`.
entry_points: Settings › Layouts; layout canvas divider
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; ET-window-manager-layout-gestures
---

story: As an operator, I can set how much room each window gets by dragging the divider between them, and trust that what I built is a layout the daemon will accept.

qa-impact: 2026-07-24 new behavior. The Settings canvas drives the divider through the runtime's own `seam-preview` math, the same code the live shell uses, so the weight vector is normalized by construction rather than validated afterwards. Flag only; the next QA cycle owns live testing.

QA impact 2026-07-25 (deep-review remediation): pointer dragging now anchors the seam at pointer
down and keeps ratio-track identities stable across updates. Flag only; the next QA cycle owns
pointer and keyboard rebalance retesting.
