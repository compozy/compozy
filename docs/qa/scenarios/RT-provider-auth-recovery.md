---
id: RT-provider-auth-recovery
area: RT
title: A turn that loses the provider sign-in waits for it and continues by itself
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: When session/prompt fails with provider_auth_required (ACP -32000 or auth wording) the turn does not fail. The session stays active, reads runtime.status "recovering" with runtime.recovery.reason "provider_auth", and the web notice reads "Waiting for the provider sign-in…". Retries back off 15s, 30s, then every minute for session.auth_recovery.window (10m). Each retry starts a fresh provider process and reloads the session. Once the sign-in returns, an untouched turn is resent and a turn that already produced output or tool calls gets a continuation, with no duplicated user message or tool call. When the window ends, the turn fails with today's provider_error guidance (handoff on user sessions), and later turns fail without waiting until one completes. Rate-limit and other failures are unchanged.
entry_points: compozy session prompt <id> "…"; compozy session status <id> -o json; compozy session events <id> -o json; compozy config set session.auth_recovery.window <d>; web session window
qa_status: planned
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-provider-error-handoff; RT-session-fallback-chain
---

Planning 2026-11-03 (#1216). Walk in an isolated lab (unique home, port, socket), never the operator's
daemon or provider login. Real provider CLIs keep the access token in memory, so deleting a lab copy of
their credential file does not interrupt a running turn. The walk therefore uses acpmock agent
`signin-agent` from `internal/testutil/acpmock/testdata/provider_error_fixture.json` as the lab's
provider. Its `fail_prompt` steps answer `-32000 Authentication required` while `.provider-credential`
is missing from the session working directory.

1. Set `session.auth_recovery.initial_delay = "10s"`, `max_delay = "20s"`, and `window = "2m"`
   (global scope) and restart the lab daemon. A workspace-scope write of the same key is rejected as
   global-only.
2. Create `.provider-credential` in the lab project, start a `signin-agent` user session, and send
   `work through a sign-in loss`. The turn reads a file, then waits 20 seconds before it uses the
   provider.
3. Move `.provider-credential` away during that wait. The turn hits the sign-in loss. Session status
   shows `runtime.status: "recovering"` and `runtime.recovery.reason: "provider_auth"`, and the attempt
   count grows. The web window shows "Waiting for the provider sign-in…".
4. Restore the file inside the window. The next retry continues the turn with no operator action, and
   the turn completes with "continued the interrupted work after the sign-in returned". `session events`
   shows `runtime_recovery_started` and `runtime_recovery_succeeded`, one user message, and the
   `signin-read-plan` tool call once.
5. Move the file away and send `sign-in lapse before work`, leaving it away past the window. The turn
   ends with one error whose `provider_error.code` is `provider_auth_required` and whose guidance matches
   `RT-provider-error-handoff`. A second prompt fails right away.
6. Restore the file and send `sign-in lapse before work` again. The turn completes.

Automated evidence at authoring time: `internal/session/manager_prompt_lifetime_test.go::TestPromptProviderAuthRecovery`
(resend, continuation, exhaustion, cancellation) and
`internal/acp/failure_test.go::TestProviderFailureDiagnosticFromErrorClassifiesProviderRecoveryActions`
(the ACP -32000 code classifies without auth wording).
