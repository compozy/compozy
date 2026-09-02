---
id: ET-web-desktop-shell-lifecycle
area: ET
title: Operate the desktop shell across workspaces and connection states
persona: Bruno
journey: J-operate-desktop-shell
expected: A fresh workspace renders one persistent desktop with menubar, dock, wallpaper, and command hint; local streams attach without requesting remote gateway tickets or logging product errors; workspace switching isolates complete window topologies; stream loss exposes an honest disconnected state, blocks unsafe mutations, and reconnect replaces the query cache from a new snapshot fence without regressing revision.
entry_points: web desktop root; workspace trigger; window-manager WebSocket stream
qa_status: pass
bug_ids: BUG-0017; BUG-20260813-desktop-shell-context-order; BUG-20260729-session-window-cross-tab-focus
fix_status: fixed
retest_status: pass
fix_commits: c3c50b6; 531b9f5; 538777e; a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-window-manager-stream-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-parity-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/08-daemon-restart-reconnected.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/23-v3-migrated-live.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-hardening.md
overlaps: ET-window-manager-public-parity; ET-window-manager-multi-client; ET-web-window-routing-lifecycle; ET-web-menubar-menu-set
---

story: As a builder, I can see the authoritative desktop state, understand when it is disconnected, and recover without mixing workspace topology or revisions.

qa-impact: 2026-07-22 window-management hard cut replaced key-level hydration with snapshot-fenced Query reconciliation and client-scoped Zustand presentation; 2026-07-24 the win-layer reserves the Dock band (windows, fullscreen fills, and floating clamps stop above the Dock), reconciled active-desktop changes (zoom, cross-desktop focus, remote switches) synthesize the same keyframe slide as an explicit switch, and rapid commands queue behind the in-flight one instead of being silently dropped; the same-day review now remeasures work-area origin after orientation and Dock layout shifts, while queued commands reject an enqueue-time binding that no longer matches the active workspace/client; 2026-07-24 the menubar chrome became a real `role="menubar"` whose items include the Compozy mark and the workspace chip (menu set tracked in `ET-web-menubar-menu-set`), so the workspace trigger is now a menu item rather than a plain button. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 frame-based decks, semantic multi-instance lookup, and state-preserving hidden
members changed shell projection and activation. Reset for the tabbed shell.

qa-impact: 2026-08-10 local stream authorization stopped probing the remote-only ticket endpoint and
now reads the listener tier explicitly. Reset for a clean-console Web and desktop-app re-walk.

qa-impact: 2026-08-20 command-palette QA repaired shell provider order, required empty wire arrays,
and the scoped settings-cache projection after each defect blocked desktop boot or window rendering.
Reset for the Task 12 shell-adjacent re-walk under `CH-untested-068-operate-desktop-shell-bruno`.

qa-impact: 2026-08-20 the palette delivery reintroduced `useDesktop` inside `useDesktopChrome`,
above the provider that hook feeds. Restored owner-atom reads. Flag only; this scenario was
already `untested`.

Walk (Task 12 re-walk):

1. Boot the daemon-served desktop — the palette registry consumer mounts under the shell provider
   and the desktop renders instead of the root error boundary.
2. Inspect the command catalog and Window Manager client payloads — command collections, client
   collections, and `global_shortcuts` are `[]`, never JSON `null`.
3. Confirm `global_shortcuts` registers the intended map and the shell reports each chord's state.
4. Confirm settings reads use the workspace/client-scoped Query-cache envelope, not a global bare
   config, so a window renders after `/agents`.

Expected evidence: boot screenshot without the root boundary; wire excerpts showing empty arrays
and `global_shortcuts`; the scoped-settings window render after `/agents`.

Walk result (2026-08-20): PASS. The workspace-scoped settings response returned required empty
arrays and an intended global shortcut without premature registration status. Agents opened in a
real desktop window with no sync warning and remained rendered after a full page reload.

qa-impact: 2026-08-20 Knowledge route projection stopped allocating a new selector result on each
external-store read. Reset to verify that opening Knowledge does not enter a render loop or break
the desktop shell.

qa-impact: 2026-09-01 stream hardening: heartbeat frames, a client stall watchdog, wake/online
verification, jittered reconnect, and 409 self-recovery replaced the sticky read-only conflict state.
Reset to re-walk "stream loss exposes an honest disconnected state ... reconnect replaces the query
cache from a new snapshot fence" from the current build; liveness assertions live in
RT-window-manager-stream-liveness.

qa-impact: 2026-09-01 a reconnect fence replaces the cache even at an equal revision, so a daemon that migrated an arrangement at load re-renders without a reload; walked B7 and P5a on the final binary.
