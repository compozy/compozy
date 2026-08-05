---
id: ET-web-session-inspector-toggle
area: ET
title: Session inspector rail closed by default with topbar toggle
persona: Bruno
journey: J-14
expected: Opening a session window shows the chat thread at full width with the Usage/Memory/Files/Vault inspector hidden. A PanelRight icon button sits immediately left of the topbar overflow (…); clicking it opens the inspector rail (inline at wide viewports, sheet drawer below the DetailInspector breakpoint) and pressing again closes it. Open/closed preference is remembered per sessionId across revisits; a fresh session defaults closed. Dismissing the narrow-viewport sheet also closes the inspector.
entry_points: web session window topbar; SessionInspector; localStorage key compozy:session:inspector:v2
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-052; RT-024; ET-web-route-chrome-topbar
---

Added by session inspector default-closed + topbar toggle (2026-07-22). Flag only — retest in the next QA cycle.

2026-07-26 state-ownership impact flag: inspector preference moved to the XState Store persistence extension and hard-cut to the `compozy:session:inspector:v2` key while retaining per-session behavior. The scenario remains untested for the next QA cycle.
