# Provider re-login recovery — 2026-11-11

Scope: issue #1481, `RT-provider-auth-recovery`, and the native-login exception in
`RT-provider-error-handoff`. Authored and verified by Codex, with Claude Opus 5.5
review. This records a targeted daemon/ACP lab, not a live OAuth account switch.

## Root cause

The default sign-in recovery window schedules eleven replacement attempts. After
exhaustion, `authRecoveryExhausted` prevents another automatic wait, while the
last provider process stays reusable. Restoring credentials on disk does not
refresh credentials retained by that process. The final error was only a prompt
failure, so canonical attention could show idle. Probe/Usage success did not
invalidate that session binding. Restarting the daemon rebuilt it and reset the
in-memory recovery budget.

## Executed journey

A fresh isolated home, provider home, HTTP port and socket ran the real daemon
with the repository's `signin-agent` acpmock fixture. The lab materialized its own
mode option and discovery command. Early discovery/mode negotiation failures
were setup failures; no product recovery result is claimed from those attempts.

The passing walk warmed the session, removed its synthetic credential, exhausted
a two-second lab recovery window, and observed `waiting-for-auth` with login
guidance. Restoring the credential and explicitly prompting the same session
completed the turn on a replacement runtime. A second revoke/restore cycle used
the successful provider auth probe to reload the session without another prompt.
Runtime generations increased; the conversation identity and daemon PID stayed
unchanged throughout both revoke/restore cycles.

The strict targeted evidence audit passed. Targeted teardown completed with `clean: true` and no surviving lab processes. Raw snapshots, prompt outputs and the
manifest remain in the lab scratch evidence. No operator credentials were copied,
revoked or restored, and the operator daemon was not restarted.

## Verification and limits

Focused Go tests cover a provider that keeps rejecting credentials until process
replacement, prompt and probe admission, active-wait wakeup, account/profile/provider
and healthy-session isolation, fresh pre-start probes, verified Usage publication,
and the Usage/config lock boundary. A Windows cross-build of the changed backend
packages passed. Repository-wide gates, race suites and E2E remain CI-owned.

No live provider OAuth or real Usage endpoint was exercised. The lab's synthetic
file credential proves the daemon/ACP flow; the sticky-credential fake owns the
in-memory retention regression. Frontend source was unchanged: existing sign-in
notices and the composer consume the corrected canonical state and login action.
