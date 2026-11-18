---
id: ET-window-tab-agent-parity
area: ET
title: Manage one tab topology through every structured surface
persona: Ada
journey: J-agent-manage-window-tabs
expected: CLI, HTTP, UDS, native tools, layout watch, and the client-bound stream expose the same open, reorder, activate, pin, move/split/unsplit, navigate, close, and reopen results on tabs; `compozy window reorder` exists; pinned-close (`window_manager_window_pinned`), stale-revision (`window_manager_revision_conflict`) and invalid-move (`window_manager_invalid_command`) failures use stable reason codes and commit no topology, history, client, or hook side effect; `window_manager_not_stacked` is no longer returned.
entry_points: compozy window reorder|activate|pin|unpin|move|close|reopen; compozy__window_reorder|activate|pin|move|close|reopen; HTTP/UDS commands
qa_status: untested
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

qa-impact: 2026-11-13 window-tabs-split S10. Groups and stacks are gone: every window is in a tab, so reorder and activate work on any window. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. From an agent session with `window_manager.write`, run `compozy__window_reorder`, `compozy__window_activate`, `compozy__window_pin` (`pinned: true`), `compozy__window_close` with `scope: "tab"` on the last window of that pinned tab (expect `window_manager_window_pinned`, revision unchanged), `compozy__window_move` with an edge placement and no target (expect `window_manager_invalid_command`), and a stale `expected_revision` with no rebase guard (expect `window_manager_revision_conflict`). Repeat with the CLI and compare codes and the `layout watch` stream (no event for rejected calls).
