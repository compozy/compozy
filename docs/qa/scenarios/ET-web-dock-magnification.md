---
id: ET-web-dock-magnification
area: ET
title: Dock proximity magnification and name tip
persona: Bruno
journey: J-operate-desktop-shell
expected: In floating presentation, moving the pointer across the dock scales and lifts nearby launchers with OpenDesign falloff (radius 96, scale amp 0.34) and shows each app name in a Tooltip above the icon; compact presentation and prefers-reduced-motion keep icons static with no tip animation requirement beyond the shared Tooltip reduce-motion behavior; New Session stays outside the magnify field but still shows its tip.
entry_points: web desktop dock
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-desktop-shell-lifecycle; ET-web-catalog-navigation
---

Added by dock hover magnification pass (2026-07-20). Flag only — retest in the next QA cycle.

Verify against `docs/design/opendesign/os/os-v2.js` (MAG_RADIUS / MAG_SCALE) and the OpenDesign dock tip hover state.
