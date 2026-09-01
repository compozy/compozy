---
id: RT-window-manager-stream-liveness
area: RT
title: Keep the live desktop stream alive without a reload
persona: Bruno
journey: J-operate-desktop-shell
expected: The window-manager WebSocket carries a heartbeat frame after every daemon ping and the shell treats silence past two heartbeats as a dead socket, reconnecting and refetching the snapshot by itself; returning to a hidden tab or regaining the network verifies the stream immediately instead of waiting for a timer; a daemon restart reconnects, re-registers the client, and refences without a page reload; a stale expected revision (409) rolls the gesture back, refreshes the snapshot, and reopens the surface for commands within a second instead of leaving every later command a silent no-op; a refused structural drop shows a short notice that clears on its own; a pointer release that lands after the layout moved carries a rebase proof and still applies when its source and target units are unchanged.
entry_points: web desktop shell; desktop app; window-manager WebSocket stream; sleep/wake; network toggle; compozy daemon restart; second browser client
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-window-manager-stream-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/05-stall-watchdog-reconnected.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/08-daemon-restart-reconnected.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-hardening.md
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-multi-client; RT-gateway-browser-stream-reconnect
---

story: As a person who leaves the desktop app open for days, I never have to reload it because the
live layout stopped updating or because every window command silently stopped working.

qa-impact: 2026-09-01 hardening pass — server `heartbeat` frames (30s, carry the latest revision),
client stall watchdog (75s), `online`/`visibilitychange` wake verification, jittered reconnect backoff,
409 conflicts self-heal (refresh + reopen, transient notice), refused commands surface a notice that
expires, and gesture commits carry `rebase` guards instead of cancelling on `stale-layout`. Walk:
observe heartbeat frames in the network inspector; block the daemon port for 90s and confirm the shell
reconnects on its own once unblocked; put the laptop to sleep for five minutes and confirm the layout is
live on wake; restart the daemon and confirm windows render without reload; from a second client
rearrange the desktop and immediately drop a window in the first client → the drop applies or rolls
back with a notice, and the next drag still works; from the CLI advance the revision twice during a
drag → the release still applies when the dragged unit is untouched.

qa-impact: 2026-09-01 walked B1–B7 on the final binary: heartbeat, 409 self-heal, heartbeat-driven refetch, online wake, stall watchdog, drop after a peer revision advance, daemon restart re-render. A reconnect fence at the same revision is now authoritative too (migration at load persists under the next revision).
