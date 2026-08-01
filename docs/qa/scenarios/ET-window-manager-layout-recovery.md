---
id: ET-window-manager-layout-recovery
area: ET
title: Validate, apply, undo, and recover declarative layouts
persona: Ada
journey: J-administer-window-manager
expected: Export returns a history-free workspace document and preserves daemon-owned `return_anchor.source_group` state for every tiled return anchor; validate and preview report stable diagnostics without writing; apply replaces the complete topology once at the expected revision; undo and redo round-trip it; global and workspace `window_layout` resources resolve with workspace precedence; malformed, executable-like, mixed resource-inline, foreign-workspace, stale, and unsupported-version documents preserve the last known-good state.
entry_points: compozy layout export|validate|apply|undo|redo|arrange; compozy__layout_*; compozy__resources_list; Settings layout editor
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260801-085219-264358-lab/qa-artifacts/qa/evidence/layout-v3.json
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-public-parity; ET-window-manager-layout-gestures; MS-configure-window-manager
---

story: As an agent or operator, I can preview and recover a complete layout without bypassing topology safety.

qa-impact: 2026-07-22 introduced versioned declarative `window_layout` resources and a single validated raw recovery path; 2026-07-23 added validated source-group recovery state to exported tiled return anchors. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 window tabs hard-cut raw layouts to v3 and added stack, navigation, pin,
and closed-entry state. Reset for the window-tabs targeted cycle.
