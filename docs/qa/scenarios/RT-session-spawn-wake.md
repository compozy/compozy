---
id: RT-session-spawn-wake
area: RT
title: Wake a parent when its child settles
persona: Cora
journey: J-respond-to-agent-attention
expected: A governed child that stops, fails, or enters a needs-you state queues one sanitized synthetic turn on its live parent by default, never interrupts an active parent prompt, and explicit notify_creator false suppresses only that child's wake with an auditable reason.
entry_points: compozy spawn --no-notify-creator; POST /api/agent/spawn over HTTP and UDS; compozy__session_spawn; parent session transcript
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; internal/session/manager_lifecycle_contract_test.go
last_report: docs/qa/reports/2026-08-24-eng-147-ttl-cleanup.md
overlaps: RT-session-wait-state; RT-session-done-presence
---

Spawn children through CLI, HTTP/UDS, and the native tool with omitted, true, and false
`notify_creator`. Exercise needs-input, failure, and stop while the parent is idle, prompting, and
stopped. Confirm one wake per cause episode, correct child identity and badge metadata, queued
delivery after a busy parent settles, redaction and the 240-character bound, plus suppression for
disabled notification, self-wake, dead parent, and failed delivery.

QA impact 2026-08-16: Task 04 added the default-on governed-child wake bridge and presence-aware
opt-out on every spawn surface. Flag only; task_08 owns execution.

QA impact 2026-08-24: ENG-147 changes the stopped-child wake classification for settled TTL
cleanup. The focused lifecycle contract verifies one clean stopped wake; a provider-backed
public-surface walk remains blocked pending an isolated ACP provider and human verification.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.

### Provider routing regression (issue #564)

Use one isolated workspace with global native provider command A, creator command B, a commandless
child, and an explicit child command C. Check that the actual ACP processes launch B/B/C, including
when the creator definition changes after its process starts. Change the selected provider and
confirm its own command wins. Change the child home/environment policy and confirm inheritance does
not override that boundary. Resume the child with a live and a stopped creator.

Spawn acceptance is not proof of future prompt authentication. Inject a provider authentication
failure at the ACP boundary and inspect the durable error and provider failure marker. Its
`provider_command_fingerprint` must match the selected route; the logical session remains available
for a later recovery prompt. Raw commands and account-directory values must not appear in markers.

Owning automated evidence: `TestSpawnProviderCommandPrecedence`,
`TestManagerIntegrationSpawnProviderCommandRouting`, and
`TestPromptGenericFailureKeepsSessionActive`. The integration case executes real ACP subprocesses
with isolated routing wrappers and SQLite; the controlled routes are not live OAuth accounts.

Routing-slice verification: [2026-09-09 issue #564 report](../reports/2026-09-09-issue-564-provider-routing.md).
`TestSpawnProviderRouteDiagnostics` additionally verifies that a startup auth failure is stopped,
retained for inspection, and correlated with exactly one fingerprint per selected/error JSON log.
This slice does not replace the unrelated wake-delivery walkthroughs above.
