---
id: ET-window-manager-layout-recovery
area: ET
title: Validate, apply, undo, and recover declarative layouts
persona: Ada
journey:
expected: Export returns a history-free workspace document and preserves daemon-owned `return_anchor.source_group` state for every tiled return anchor; validate and preview report stable diagnostics without writing; apply replaces the complete topology once at the expected revision; undo and redo round-trip it; global and workspace `window_layout` resources resolve with workspace precedence; malformed, executable-like, mixed resource-inline, foreign-workspace, stale, and unsupported-version documents preserve the last known-good state.
entry_points: agh layout export|validate|apply|undo|redo|arrange; agh__layout_*; agh__resources_list; Settings layout editor
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-public-parity; ET-window-manager-layout-gestures; MS-configure-window-manager
---

story: As an agent or operator, I can preview and recover a complete layout without bypassing topology safety.

qa-impact: 2026-07-22 introduced versioned declarative `window_layout` resources and a single validated raw recovery path; 2026-07-23 added validated source-group recovery state to exported tiled return anchors. Flag only; the next QA cycle owns live retesting.
