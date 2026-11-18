---
id: ET-window-manager-layout-recovery
area: ET
title: Validate, apply, undo, and recover declarative layouts
persona: Ada
journey: J-administer-window-manager
expected: Export returns a history-free version 6 workspace document (`{version: 6, workspace_id, desktops, windows, overrides}`); validate and preview report stable diagnostics without writing and accept documents of version 4, 5 and 6; apply replaces the complete topology once at the expected revision and converts a version 4 or 5 document to tabs; undo and redo round-trip it; global and workspace `window_layout` resources resolve with workspace precedence; malformed or unsupported-version documents fail with `window_manager_invalid_topology` (`topology.unsupported_version`) and write nothing.
entry_points: compozy layout export|validate|apply|preview|undo|redo; compozy__layout_export|validate|apply; compozy layout-profile put
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260801-085219-264358-lab/qa-artifacts/qa/evidence/layout-v3.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-public-parity; ET-window-manager-layout-gestures; MS-configure-window-manager
---

story: As an agent or operator, I can preview and recover a complete layout without bypassing topology safety.

qa-impact: 2026-07-22 introduced versioned declarative `window_layout` resources and a single validated raw recovery path; 2026-07-23 added validated source-group recovery state to exported tiled return anchors. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 window tabs hard-cut raw layouts to v3 and added stack, navigation, pin,
and closed-entry state. Reset for the window-tabs targeted cycle.

2026-08-23 qa-impact (Profiles): **reset from `pass` to `untested`** — two things this row asserts
changed. The `window_layout` resource scope word `global` was hard-cut to `user` and a profile layer
joined the precedence, so "global and workspace resolve with workspace precedence" must be re-walked
as user → profile → workspace → workspace-profile. Separately, window-manager snapshots now
partition per profile, so export, apply, undo, and redo operate on the acting profile's topology.
Re-walk the resource-precedence and recovery arms in a non-default profile.
`ET-profile-desktop-restoration` owns per-profile restoration itself.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Documents are version 6; `return_anchor` no longer exists. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Build two desktops with a split tab and a pinned tab. `compozy layout export -o json > v6.json`: `version` is 6, no history or closed entries. `compozy layout validate --file v6.json` passes with no write (revision unchanged). Edit `version` to 7: validate fails with `topology.unsupported_version`. Validate and apply a version 5 document exported by v0.6 (keep one from an older lab, or the fixture used by `ET-layout-editor-import-legacy-document`): it applies as tabs, then `compozy layout undo` restores the previous layout and `redo` re-applies it. Export again: `version` is 6.
