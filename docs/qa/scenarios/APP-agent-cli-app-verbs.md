---
id: APP-agent-cli-app-verbs
area: APP
title: Drive the desktop surface end-to-end through compozy app
persona: Ada
journey: J-desktop-agent-headless
expected: The full lifecycle — status before install (installed:false), open (launch/focus), transitional provisioning/updating states verbatim, attached running:true with runtime fields, open /settings navigation, update --check/--apply app|runtime, diagnose log paths, and running:false after kill — is deterministic, schema-valid `-o json`, with named error codes for every failure.
entry_points: compozy app status|open|update|diagnose -o json; app.sock control socket
qa_status: blocked-verify
bug_ids: BUG-20260810-app-control-timeout; BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: 0805f649; f081a1e
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-status-before-daemon.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-status-attached.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-control-product.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md
overlaps: APP-update-recovery-state
---

PRD stories: US-019 (AC-1 open, AC-2 structured status, AC-3 deterministic not-installed error;
EC-1 open with target path parity; EC-2 transitional states truthful). BR-20. Test IDs: E2E-017,
E2E-024, E2E-025; IT-021, IT-031; UT-071–UT-076, UT-082, UT-109–UT-111.

Per-OS evidence (N-004): the full E2E-017 lifecycle transcript (every `-o json` payload validated
against `desktop/schema/app-state.schema.json`) runs on Linux and Windows; macOS records the same
verb sequence in the scripted-manual smoke (the control socket is AF_UNIX on all three OSes —
socket permission `0600` asserted where the platform exposes it). E2E-024 (headless app + runtime
update) runs against the staging fixture feed on at least one scripted OS with the quiesce
readout captured; the socket-absent (`app_not_running`) and unresponsive
(`app_control_unavailable`) branches are walked per OS.
