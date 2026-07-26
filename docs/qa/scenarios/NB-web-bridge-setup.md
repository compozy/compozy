---
id: NB-web-bridge-setup
area: NB
title: Complete bridge setup in the Web
persona: Tessa
journey: J-complete-web-bridge-setup
expected: A browser-first operator can create a disabled Slack bridge, copy its daemon-generated manifest, follow daemon-derived setup state and inline verification remediation, register Telegram webhooks, and distinguish dry-run target checks from real test sends. At 320px, the create dialog and detail panel reflow their step navigation, provider cards, actions, secret inputs, complete secret references, and provider/config metadata without horizontal scrolling or clipping.
entry_points: Web bridges create dialog; Web bridge detail panel
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/screenshots/ch055-create-handoff.png; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/screenshots/ch055-failed-remediation.png; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/screenshots/ch055-send-result.png
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps: NB-bridge-provider-setup; NB-026; NB-027; NB-036; NB-037; NB-038; NB-039
---

A first-time adopter completes provider setup through the Web without losing the distinction between
configuration checks and real delivery.

Added by the Hermes bridge Task 05 impact flag. Task 09 assigned it to `J-complete-web-bridge-setup` and `CH-web-bridge-setup`; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: browser create, daemon manifest copy, bindings, inline failed-check remediation, refresh, dry-run, and real-send all completed; API/CLI/provider readbacks confirmed durable truth.

Phase D responsive impact flag 2026-07-13: the bridge detail panel now reflows secret fields, complete vault references, and provider/config metadata at 320px instead of forcing horizontal overflow. Status reset to `untested`; historical browser QA evidence remains intact. No QA retest ran.

Phase D source-freeze impact flag 2026-07-13: the bridge create dialog now also reflows step navigation, provider cards, and actions at 320px. Status remains `untested`; historical browser QA evidence remains intact. No QA retest ran.
