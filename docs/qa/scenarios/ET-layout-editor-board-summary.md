---
id: ET-layout-editor-board-summary
area: ET
title: Read exact layout occupancy from the canvas summary
persona: Bruno
journey: J-administer-window-manager
expected: Settings Layouts shows the active desktop's exact tiled, floating, minimized, and zoomed counts beside the reference dimensions; zero optional counts stay hidden, a visible zoom count is labeled with the structural eyebrow treatment, and the summary never substitutes a hardcoded count for the draft it is describing.
entry_points: Settings > Layouts; layout canvas board summary
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/screenshots/08-layout-board-zoom-count.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-review-fixes.md
overlaps: ET-window-manager-layout-recovery; MS-configure-window-manager
---

story: As an operator reviewing a layout, I can trust the compact canvas summary to describe the active desktop rather than a placeholder value.

qa-impact: 2026-09-01 review remediation renders the calculated zoom count and applies the canonical eyebrow utility to its structural label. Added for a focused current-head walk.

qa-impact: 2026-09-01 focused walk selected the workspace-scoped Layouts editor and verified the live active-desktop summary (`1 tiled`, `0 floating`, `1 zoomed`, `1440 × 900 reference`). The owning component regression supplies a two-zoom draft and proves the label renders `2 zoomed`, which distinguishes the calculated value from the former constant.
