---
id: ET-extension-host-api-grant-continuity
area: ET
title: Preserve a live extension's Host API grant across install reloads
persona: Ada
journey: J-extension-distribution
expected: An installed extension that receives `sessions/list` in its initialize handshake can call that Host API method immediately and after later extension installs reload the manager; authorization remains bound to the live session while rate limits, ownership checks, and public diagnostics retain the stable extension name, and a stale generation cannot use the replacement session's grant.
entry_points: `compozy extension install`; `POST /api/extensions` (HTTP+UDS); extension initialize handshake; extension Host API `sessions/list`; `compozy extension status`
qa_status: pass
bug_ids: BUG-20260803-extension-session-grant-denied
fix_status: fixed
retest_status: pass
fix_commits: pending final whole-diff commit
evidence: internal/extension/reference_integration_test.go; internal/extension/host_api_test.go; /home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/evidence/extension-host-api/verification.json; /home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-03-extension-host-api-grant-continuity.md
overlaps: ET-017; ET-extension-dev-reload-loop; ET-extension-manifest-v2-surfaces
---

Added by the restarted Go audit after the complete tagged Extension suite exposed a live-runtime
authorization regression. The Manager registered and checked a nonce-scoped session grant, but the
shared Host API handler repeated the check with the public extension name. Valid runtimes therefore
received `Capability denied` even though their initialize handshake granted `sessions/list`.

Run one isolated daemon and install the Go `secret-guard` and TypeScript `prompt-enhancer` reference
extensions through public install surfaces. For each runtime, retain the initialize handshake and
the first `sessions/list` result. Perform another public lifecycle mutation so the manager replaces
the installed runtime sessions, then repeat the call from the newly active generations. Both
languages must receive a successful result, and status must remain active and healthy.

Prove identity separation rather than only success: the live runtime uses its private session grant;
public status, ownership, and rate-limit diagnostics use the stable extension name; and an older
generation cannot call with the replacement generation's authority. Stop the daemon and retain clean
teardown evidence with no surviving extension process.

QA result 2026-08-03: pass. In a fresh isolated lab, the Go and TypeScript runtimes received
`sessions/list`, completed the call before and after manager reloads, remained active and healthy in
CLI/HTTP status, and continued to reject ungranted `sessions/create` with JSON-RPC `-32001`.
Mandatory teardown recorded `clean=true` with no surviving process. The generic release-grade audit
remains blocked by nine intentionally out-of-scope minimums plus the deferred workstream gate; this
does not change the focused scenario verdict.
