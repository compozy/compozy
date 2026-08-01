---
id: ET-window-tab-agent-parity
area: ET
title: Manage one tab topology through every structured surface
persona: Ada
journey: J-agent-manage-window-tabs
expected: CLI, HTTP, UDS, native tools, layout watch, and the client-bound stream expose the same group, reorder, activate, pin, navigate, close, and reopen results; not-stacked, pinned-close, and stale-revision failures use stable reason codes and commit no topology, history, client, or hook side effect.
entry_points: compozy window list|group|activate|pin|unpin|navigate|close|reopen; compozy layout watch; compozy__window_group|window_reorder|window_activate|window_pin|window_reopen; /api/workspaces/{workspace_id}/window-manager over HTTP and UDS
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-01-cli-route-parity.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-public-parity; ET-window-manager-hooks-resources; ET-window-tab-pinning
---

Derived from J-agent-manage-window-tabs steps 1-3. Covers every agent-manageability plane,
deterministic errors, workspace isolation, client presentation fences, and hook silence on rejection.
