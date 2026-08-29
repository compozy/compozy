---
id: APP-agent-cli-app-verbs
area: APP
title: Drive the desktop surface end-to-end through compozy app
persona: Ada
journey: J-desktop-agent-headless
expected: The full lifecycle — status before install, open or focus, retry, schema-v2 transitional state, navigation, `compozy update` handoff, redacted diagnostics, consent-gated bundle export, and stopped state — is deterministic structured output with named errors.
entry_points: compozy app status|open|retry|diagnose -o json; compozy update [--check|--cancel] -o json; app.sock control socket
qa_status: blocked-verify
bug_ids: BUG-20260810-app-control-timeout; BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: 0805f649; f081a1e
evidence: docs/qa/reports/2026-08-27-pr-494-desktop-liveness.md
last_report: docs/qa/reports/2026-08-27-pr-494-desktop-liveness.md
overlaps: APP-update-recovery-state
---

PRD stories: US-019 (AC-1 open, AC-2 structured status, AC-3 deterministic not-installed error;
EC-1 open with target path parity; EC-2 transitional states truthful). BR-20. Test IDs: E2E-017,
E2E-024, E2E-025; IT-021, IT-031; UT-071–UT-076, UT-082, UT-109–UT-111.

Per-OS evidence: the full E2E-024/E2E-025 lifecycle transcript (every `-o json` payload validated
against `desktop/schema/app-state.schema.json`) runs on Linux and macOS. The control socket is
AF_UNIX on both release OSes and permission `0600` is asserted. E2E-026/E2E-027 use the mock GitHub
Release and channel-beta fixture with the quiesce readout captured; socket-absent
(`app_not_running`) and unresponsive (`app_control_unavailable`) branches are walked per OS.

QA impact 2026-08-11: `compozy app diagnose` now returns the redacted shared report, falls back to
the persisted report only when the desktop app is not running and its control socket is absent, and
creates a local archive only with `--bundle --yes`. Reset to verify report schema, offline fallback,
the unresponsive-socket error, explicit consent, default and selected archive paths, and that no
archive includes raw paths, unredacted logs, `compozy.log`, databases, configuration, credentials,
sessions, or transcripts. The archive may contain only `manifest.json` plus bounded, redacted
current-boot `desktop.log` and `desktop-bootstrap.jsonl` tails. The isolated macOS walk passed live
and offline diagnose, consent, allowlist, permissions, stale-socket, and no-clobber legs. The full
packaged status/open/update matrix and shipping OS coverage remain blocked for artifact verification.

QA impact 2026-08-16: app state moved to schema v2 and the runtime/app operation moved to
`compozy update`. Reset for the Task 07 headless walk.

QA impact 2026-08-27: desktop liveness now comes from an authenticated `app.sock` response instead
of PID start-time comparison. Reset to verify `running:true` while the live control channel responds,
`open /settings` uses that channel, and a stopped or unresponsive app reports `running:false`.
