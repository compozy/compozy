---
id: RT-session-spawn-wake
area: RT
title: Wake a parent when its child settles
persona: Cora
journey: J-respond-to-agent-attention
expected: A governed child that stops, fails, or enters a needs-you state queues one sanitized synthetic turn on its live parent by default, never interrupts an active parent prompt, and explicit notify_creator false suppresses only that child's wake with an auditable reason.
entry_points: compozy spawn --no-notify-creator; POST /api/agent/spawn over HTTP and UDS; compozy__session_spawn; parent session transcript
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-wait-state; RT-session-done-presence
---

Spawn children through CLI, HTTP/UDS, and the native tool with omitted, true, and false
`notify_creator`. Exercise needs-input, failure, and stop while the parent is idle, prompting, and
stopped. Confirm one wake per cause episode, correct child identity and badge metadata, queued
delivery after a busy parent settles, redaction and the 240-character bound, plus suppression for
disabled notification, self-wake, dead parent, and failed delivery.

QA impact 2026-08-16: Task 04 added the default-on governed-child wake bridge and presence-aware
opt-out on every spawn surface. Flag only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
