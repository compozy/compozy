---
id: RT-provider-auth-recovery
area: RT
title: A turn that loses the provider sign-in waits for it and continues by itself
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: When session/prompt fails with provider_auth_required (ACP -32000 or auth wording) the turn does not fail. The session stays active, reads runtime.status "recovering" with runtime.recovery.reason "provider_auth", and the web notice reads "Waiting for the provider sign-in…". Retries back off 15s, 30s, then every minute for session.auth_recovery.window (10m). Each retry starts a fresh provider process and reloads the session. Once the sign-in returns, an untouched turn is resent and a turn that already produced output or tool calls gets a continuation, with no duplicated user message or tool call. When the window ends, the session shows waiting-for-auth with login guidance. After credentials return, the next prompt replaces the stale process and reloads the conversation. A successful auth probe or Usage collection also recovers matching auth-failed sessions in the same profile, provider and account route; healthy sessions stay intact. Rate-limit and other failures are unchanged.
entry_points: compozy session prompt <id> "…"; compozy session status <id> -o json; compozy session events <id> -o json; compozy config set session.auth_recovery.window <d>; web session window
qa_status: partial
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence: docs/qa/evidence/2026-11-03-issue-1216-auth-recovery/events-summary.json; docs/qa/evidence/2026-11-03-issue-1216-auth-recovery/signin-wait.png
last_report: docs/qa/reports/2026-11-12-issue-1557-recovery-model-list.md
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

Verification 2026-11-03: isolated daemon and ACP subprocess walk passed recovery, expiry, immediate
followup failure, restored sign-in and Web wait/automatic completion. Strict audit passed with no
blockers or warnings; teardown was clean. The provider and credential were synthetic, so this does
not establish live OAuth account-switch behavior. See the linked run report for timestamps and limits.

Review follow-up (#1229): workspace TOML containing any `session.auth_recovery` value must fail
with a global-only error. ACP -32000 takes precedence over model wording. Only `login` failures
enter the sign-in wait; `bind_secret` and `inspect` failures immediately show their existing
provider guidance (handoff for user sessions). Canceling a later turn must not rearm an exhausted provider;
a successful turn or an effective provider change does. Process-exit finalization must allow the
whole configured auth window, including replacement startup time. The expiry remains a hard
boundary: no replacement may start at or after it.

Follow-up verification: focused Go suites cover these invariants. The previous live lab evidence
above predates this follow-up; a new scenario walk and broad gates are deferred under the host-load
restriction.

Environment isolation regression (#1368): use synthetic parent credentials only. Run
`go test ./internal/session -run TestProviderEmptyEnvironmentLaunch -count=1`
and `go test ./internal/acp ./internal/subprocess ./internal/procutil -run 'TestLocalLauncherEnvironmentInheritance|TestManagedCommandDropsInternalHandoffEnv|TestDaemonEnvironmentInheritance' -count=1`.
Both filtered and isolated policies can yield a non-nil empty environment when their
input contains only excluded variables. Removing the final optional `PATH` target also
leaves a non-nil empty environment; the real child must report no parent credential.
Ordinary session starts inject session/profile/workspace/agent identity entries, so filtering
alone does not normally empty the final session environment. Reserved `COMPOZY_` targets cannot be declared, so optional credentials cannot remove
those identity entries. The focused assembly test omits session identity deliberately;
the lower-level launch boundary must remain safe for explicit empty callers.
ACP executable discovery failure after internal-only filtering is covered with an injected
executable lookup failure and a real child. Nil inherits ordinary parent variables;
explicit empty and internal-only environments never restore them. Internal handoff variables
remain absent in ordinary children. Terminal maps remain overlays on filtered inheritance.
This focused subprocess evidence does not re-walk the OAuth recovery scenario above.

Model-discovery follow-up: `go test ./internal/modelcatalog -run TestLiveDiscoveryEnvironmentInheritance -count=1` verifies explicit empty `BaseEnv` through source construction and filtering, plus direct executor launches, using real children. Only unspecified discovery input inherits the parent sentinel.

Discovery coverage also checks direct nil-environment inheritance and a real ACP inspection with the canonical fixture driver. Windows retains Go's documented critical `SYSTEMROOT` addition; empty-environment isolation does not disable required OS startup variables.

Recovery after expiry (#1481): revoke the synthetic credential, exhaust the bounded wait, and verify
`waiting-for-auth` plus `next_action: login`. Restore it and explicitly retry in the same session;
the provider generation increases and the prompt completes with the daemon PID unchanged. Repeat the
failure and restore, then run the provider auth probe: the session reloads before another prompt.
A verified successful Usage collection must signal the same recovery; a refused or stale publication
must not. Keep a healthy session and a different provider/profile/account route as isolation controls.
The owning `TestPromptProviderAuthRecovery` uses a provider that retains invalid credentials until
process replacement, covering the failure that a file-presence-only mock cannot reproduce.

Pinned-model sign-out regression (#1557): keep the full Claude model ID pinned.
While the synthetic login is absent, make the resumed adapter advertise only its
offline aliases. Native `auth status` must report a login wait before model
negotiation, preserving the auth window rather than reporting model unavailability.
Restore the login and account model list, then run the provider auth probe; the
matching recovery must retry promptly, retain the conversation, and finish without
a daemon restart. A runtime drop that discovers missing login receives the auth
window as well. Verified Usage publication uses the same wake path; an unverified
ownership-change log alone is not proof of a valid login.

After a successful login, deliberately request a genuinely unavailable model.
After its bounded retries, recovery must emit the configuration rejection once, without native sign-in or
permission guidance. Child attention must distinguish provider sign-in from a
pending permission. Do not substitute a family alias for the pinned full ID.

Authenticated stale-picker regression (#1557): keep native auth status successful
and pin `claude-opus-5-5` or `claude-fable-5-2`. Rebuild the lab session runtime while
the adapter's built-in picker contains only older versions and family aliases.
The exact pin must be available through the session-local Claude startup option
and confirmed by ACP before work resumes. Repeat with a loaded current value that
already equals the pin but is absent from the picker: no model setter is needed.
Refused, ignored or unconfirmed selections must remain configuration failures,
without a login/permission badge or a silently substituted alias.

This additional case was observed on the signed-in operator runtime after the
earlier synthetic walks. Its corrective regression runs in the existing ACP
start/resume suite through CI. The operator's subsequent CI-only rule prohibits
another local lab walk; the earlier pass below covers restoration of both login
and the full model list, not this additional case.


Verification 2026-11-12 (#1557): the complete isolated final-build walk passed
mid-work loss and automatic continuation, expiry, immediate refusal, explicit
retry, proactive probe reload, and bounded genuine model rejection with failed
attention. The full pinned model and conversation stayed unchanged; the auth-probe
wake completed in 0.202 seconds. Strict targeted audit: zero blockers/warnings.
Synthetic login and ACP subprocesses establish the daemon flow, not live OAuth.
