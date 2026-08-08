---
id: RT-gateway-browser-stream-reconnect
area: RT
title: Reconnect remote browser streams with fresh tickets
persona: Iris
journey: J-expose-and-pair-gateway
expected: Every remote SSE and WebSocket connection or reconnect mints and consumes a fresh single-use ticket, ordinary network or server failures remain recoverable, and device revocation alone produces the terminal access-ended state with no cached data.
entry_points: Paired private or public Gateway UI; session, task, loop, bridge, extension, dashboard, Network, and window-manager live views
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/08-revoked-browser-device.png
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-paired-device; RT-gateway-public-ui-consent; RT-gateway-remote-cli-profile
---

Exercise first connect, reconnect after network loss, an expired ticket, a reused ticket, a 5xx, and
revocation during live work. A device credential must never appear in a query string; ticket values
must not survive in UI state, logs, events, or cached data. Access teardown cancels and removes
gateway-scoped queries before rendering the terminal state.

QA walk 2026-08-07: local browser revocation rendered the terminal state and removed the active
device. A real remote stream and ticket reconnect remain blocked because no authorized provider
address was available; no local transport was substituted for that leg.
