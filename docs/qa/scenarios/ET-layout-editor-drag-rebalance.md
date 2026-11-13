---
id: ET-layout-editor-drag-rebalance
area: ET
title: Rebalance a split by dragging its seam in the Settings layout editor
persona: Bruno
journey: J-administer-window-manager
expected: On Settings › Layouts › Project layout, the canvas draws the picked tab's split with the shell's own split renderer; dragging the seam between two windows moves it under the pointer, shows a live share readout such as "⅓ · ⅔" or "38% · 62%" that turns brand-coloured on a half, third or quarter, and changes only the two windows beside the seam; arrow keys on the focused seam nudge it by 2% and double-click balances the pair; the inspector's Balance list follows; Review changes is accepted without `topology.split_weight_sum` and Apply layout reproduces the shares in the shell.
entry_points: Settings › Layouts › Project layout; layout canvas seam; selection inspector › Balance
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager; ET-window-manager-layout-gestures
---

story: As a person running agent work, I can set how much room each window gets by dragging the divider between them, and trust that what I built is a layout the daemon will accept.

qa-impact: 2026-07-24 new behavior. The Settings canvas drives the divider through the runtime's own `seam-preview` math, the same code the live shell uses, so the weight vector is normalized by construction rather than validated afterwards. Flag only; the next QA cycle owns live testing.

QA impact 2026-07-25 (deep-review remediation): pointer dragging now anchors the seam at pointer
down and keeps ratio-track identities stable across updates. Flag only; the next QA cycle owns
pointer and keyboard rebalance retesting.

qa-impact: 2026-11-13 window-tabs-split S10. The editor no longer has a free canvas, group
frames or a 1440×900 reference; it edits desktops → tabs → splits, and the canvas is the shell's split
view for one tab. Seam drag, keyboard nudge and double-click balance are the current controls.
Walk: preconditions — a project whose desktop has a tab split into three windows side by side (in
the shell, drag one tab onto the edge of another twice, or apply such a layout). Open Settings ›
Layouts, pick that desktop and tab in the Desktops and Tabs strips. 1) Drag the seam between the
first two windows: the readout follows the pointer, reads a named fraction (brand colour) near ¼, ⅓,
½, ⅔ or ¾ and a percentage between them; the third window's width does not change. 2) Click a window
and read the inspector's Balance list: the shares match the canvas and add up to the whole. 3) Tab
to a seam and press ArrowRight five times: it moves about 10% and the readout is not shown (no
drag). 4) Double-click a seam: the pair splits ½ · ½ of their combined share. 5) Press Review
changes: the status reads "Checked · changes …" and no `topology.split_weight_sum` diagnostic
appears; press Apply layout and confirm the live tab shows the same shares. Reset for the current
build.
