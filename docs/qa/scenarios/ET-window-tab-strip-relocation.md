---
id: ET-window-tab-strip-relocation
area: ET
title: Navigate peer views from the toolbar strip
persona: Cora
journey: J-organize-tabbed-work
expected: Tasks and Marketplace render peer-view navigation as the leading toolbar-strip group in views, filters, spacer, display-mode order; the topbar head remains title plus controls only, drill-in back and crumbs remain reachable, and routes without peers render no invented views group.
entry_points: web /tasks; web /marketplace; task and marketplace drill-in routes; packages/site configuration docs
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-06-route-reload-general.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-web-window-routing-lifecycle; ET-web-route-chrome-topbar
---

Derived from J-organize-tabbed-work navigation. Covers discoverability, plain-language hierarchy,
keyboard reachability, absent-view truthfulness, and the D3 design-system migration.
