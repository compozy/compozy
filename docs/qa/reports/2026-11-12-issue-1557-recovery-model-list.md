# Native sign-in recovery with a pinned Claude model

Scope: issue #1557 and `RT-provider-auth-recovery`. Persona: Rafa, operating
sessions through CLI and HTTP. The lab has its own daemon home, provider home,
HTTP port, and socket. Credentials and the ACP provider are synthetic; no operator
login, installed binary, or running daemon is changed.

## Evidence correction and baseline

The issue's corrected timeline places all four failed production retries before
the new account became active. The session was canceled before a post-login retry;
that evidence does not establish permanent failure after login.

The isolated baseline confirmed that a session pinned to `claude-opus-5-5` can
recover after the synthetic sign-in and full model list return, preserving its
conversation and daemon PID. While signed out, its resumed adapter advertised only
`default`, `opus`, `sonnet`, `haiku`, and `fable`, and the driver rejected the pinned
ID as unavailable. The baseline therefore confirms misleading signed-out model
negotiation, rather than proving that the full ID must be replaced by an alias.

## Matrix

| Journey | Status | Evidence |
| --- | --- | --- |
| Baseline: full model, offline list, sign-in restored | Pass | [Baseline facts](../evidence/2026-11-12-issue-1557-recovery.json) |
| Corrected: loss during work, native login wait, probe wake, continuation | Pass | [Continuation receipt](../evidence/2026-11-12-issue-1557-recovery.json) |
| Exhaustion, explicit retry and proactive probe recovery | Pass | [Expiry and proactive probe receipts](../evidence/2026-11-12-issue-1557-recovery.json) |
| Genuine unavailable model after login, live runtime | Pass | [Configuration rejection receipt](../evidence/2026-11-12-issue-1557-recovery.json) |
| Runtime exit, sign-in recovery, then genuine model rejection | Pass | [Corrective finalization receipt](../evidence/2026-11-12-issue-1557-recovery.json) |
| Cancellation during runtime-exit backoff | Pass | [Cancellation receipt](../evidence/2026-11-12-issue-1557-recovery.json) |

## Implementation

Claude's built-in native auth route now uses `claude auth status` before ACP
startup. A signed-out provider is classified as needing login before its fallback
model list can cause a configuration refusal. Explicit custom auth commands and
provider isolation still use the existing route/environment resolver.

A runtime-exit retry that discovers a login failure enters the bounded auth
recovery window. Verified auth/Usage proofs wake matching in-flight recoveries,
including runtime-exit waits, without changing healthy sessions or other routes.
Recovery probes bypass stale cached verdicts. Model-list rejections keep the bounded retry window, since credential presence
alone does not prove token validity. If attempts exhaust, the last actual failure
owns the terminal diagnostic and persisted badge instead of the original auth failure. Child-session attention distinguishes native sign-in from permission.

## Validation and limits

Focused ACP subprocess, recovery and child-attention regressions passed on the
implementation commit. Scoped lint passed with zero issues. The unmodified baseline
and the complete final-build daemon walks passed; the final continuation finished
0.202 seconds after the verified native-login probe. The same daemon PID and native
conversation ID survived the final-build revoke/restore cycles. Full tests, race suites and repository gates remain CI-owned. No live
OAuth account switch, real Usage endpoint, or rendered Web evidence is claimed.

## Review repairs

The first Opus review found cancellation losing its canceled terminal after a
runtime-exit-to-auth transition, premature termination on an offline model list,
and final failure ownership being lost to the old process watcher. Canonical
regressions now cover cancellation, mismatch-then-success, bounded final
configuration failure, and auth exhaustion after the old watcher callback runs.
The production path preserves the retry budget and detaches only already-dead
runtimes before settling a login wait. The second Opus review and Greptile
independently found a fourth route: auth-window
exhaustion after a dead runtime and genuine model rejection released the binding
before fatal finalization. The corrective change releases a dead binding only for
a real auth-required or canceled terminal; protocol failures keep it until finalization owns
the stopped/failed state. The existing rejection regression now covers both live
and dead starting runtimes, failing before the correction and passing after it.
CodeRabbit also identified stale recovery state when canceling runtime-exit
backoff. Its regression failed before the fix; cancellation now clears recovery
and derives readiness from process liveness before delivering the terminal. The
model retry regression explicitly pins and verifies `claude-opus-5-5` on every
replacement. Final independent-review dispositions and current-head CI belong to
the PR discussion.

## Final targeted walk

The final build passed loss of login after a tool result, automatic continuation,
window exhaustion, immediate follow-up refusal (HTTP 422), explicit restored-login
retry, proactive probe reload, and a genuinely unavailable model after local login
returned. The genuine rejection spent the bounded retries, emitted one protocol
error, and persisted a failed badge with the model detail. The exact full model was
never replaced by a family alias. The strict targeted audit passed with zero
blockers and zero warnings. Raw streams, snapshots, manifest and focused-check
receipts remain in the isolated lab scratch evidence.

The corrective daemon walk independently passed runtime exit, sign-in loss, and
a genuine model rejection after sign-in: the session ended stopped with a failed
badge and `protocol_failure`. A reviewed-build cancellation walk confirmed the
public cancel outcome, unbound runtime, cleared recovery, and no failure. Its
strict targeted audit also passed with zero blockers and warnings.

Early lab setup/script attempts are not product results: an invalid fixture agent
frontmatter was corrected before startup; response-envelope, idempotency-key,
managed-CLI identity, public stream-envelope, and script bookkeeping mistakes were corrected and the full
final-build walk was repeated. A real product defect exposed by the intermediate
walk (configuration failure read as done) was repaired before that final walk.

The test-shape heuristic passes the changed recovery and child-wake suites. The
ACP start/helper and config files retain exactly the baseline findings (16, 1 and
16 respectively); no new heuristic finding was introduced. Broad/race/E2E checks
remain current-head CI responsibilities.

Both targeted lab teardowns completed with `clean: true`: only each lab's registered daemon
and its verified lab-owned child processes were stopped. No surviving lab process
or copied provider resource remains.
