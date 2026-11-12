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

| Journey                                                                  | Status  | Evidence                                                           |
| ------------------------------------------------------------------------ | ------- | ------------------------------------------------------------------ |
| Baseline: full model, offline list, sign-in restored                     | Pass    | Baseline stream, status and final snapshot in isolated lab scratch |
| Corrected: loss during work, native login wait, probe wake, continuation | Pending |                                                                    |
| Exhaustion, explicit retry and proactive probe recovery                  | Pending |                                                                    |
| Genuine unavailable model after login                                    | Pending |                                                                    |

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

Focused ACP subprocess and session recovery regressions passed. The baseline used
the unmodified daemon; corrected real-daemon walks and final scoped lint are in
progress. Full tests, race suites and repository gates remain CI-owned. No live
OAuth account switch, real Usage endpoint, or rendered Web evidence is claimed.


## Review repairs

The first Opus review found cancellation losing its canceled terminal after a
runtime-exit-to-auth transition, premature termination on an offline model list,
and final failure ownership being lost to the old process watcher. Canonical
regressions now cover cancellation, mismatch-then-success, bounded final
configuration failure, and auth exhaustion after the old watcher callback runs.
The production path preserves the retry budget and detaches only already-dead
runtimes before settling a login wait. The second Opus review is in progress.
