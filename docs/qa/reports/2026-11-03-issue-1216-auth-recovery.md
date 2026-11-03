# QA Run Report — 2026-11-03 — Provider sign-in recovery

- **Scope:** PR #1229, `RT-provider-auth-recovery` and the auth leg of `RT-provider-error-handoff`.
- **Cadence tier:** targeted.
- **Build:** `89b1f72ab99a47e9783e883b799f2fec7fe40083`; later changes add fixture model/effort options and format documentation.
- **Environment:** isolated daemon, home, socket and HTTP port; real ACP subprocess using `acpmock` with a removable stand-in credential. This proves daemon/ACP integration, not a live provider OAuth account switch.
- **Started:** 2026-11-03T21:24:32Z. **Status:** closed.

## Session Matrix & Results

| Journey / Scenario | Persona | Probe | Status |
| --- | --- | --- | --- |
| J-15 / RT-provider-auth-recovery | Rafa | Restore sign-in during interrupted work | Pass |
| J-15 / RT-provider-auth-recovery | Rafa | Exhaust the wait, retry, restore sign-in | Pass |
| J-15 / RT-provider-auth-recovery | Rafa | Observe the waiting notice in Web | Pass |

## Session Debriefs

The CLI prompt began at 21:24:32Z. The lab credential was moved away at 21:24:38Z
and restored at 21:25:37Z. The prompt completed at 21:25:46Z without another
operator prompt. Its event output contains one tool call/result pair before
recovery, three recovery attempts and the final continuation message followed by
`done`. Recovery did not repeat the tool call in this fixture.

The first status capture used `session status`, which does not expose the
`runtime` projection. A subsequent public session GET observed
`runtime.status: recovering` and reason `provider_auth`. This capture mistake
does not establish a product defect.

## Remaining Probes and Evidence

The persisted event stream confirms six auth recovery attempts from 21:31:13Z
through 21:33:06Z, followed by one `runtime_recovery_exhausted` and one
`provider_auth_required` error with `next_action: handoff`. Two subsequent prompts
failed in zero to one second. After restoration, the next prompt completed at
21:35:48Z. The original command reader was canceled, but the daemon continued the
accepted turn; these results come from the persisted session events.

At 21:43Z, the real Web displayed "Waiting for the provider sign-in…" and
"Retrying automatically · Attempt 2 of 6" while the session remained Working.
Restoring the credential completed the turn at 21:44Z. The notice disappeared,
the session became Idle, and "finished after the sign-in returned" appeared.

Local evidence (gitignored sidecar paths):

- `docs/qa/evidence/2026-11-03-issue-1216-auth-recovery/events-summary.json`
- `docs/qa/evidence/2026-11-03-issue-1216-auth-recovery/signin-wait.png`

The screenshot was retained locally: automatic approval review rejected external
R2 publication. No screenshot was published. All provider messages and credential
files in this lab were synthetic; no operator provider login was changed.

## Verification and Final Status

- **Targeted integration:** PASS for recovery, expiry, immediate followup failure,
  credential restoration, Web wait notice and automatic Web completion.
- **Strict evidence audit:** PASS, zero blockers and zero warnings, using
  `audit-qa-evidence.py --qa-output-path "$QA_OUTPUT_PATH" --strict`.
- **Lab teardown:** `teardown.json` records `clean: true`, completed at
  2026-11-03T21:47:02Z, with no provider resource errors. Only this lab was reaped.
- **Local build:** Web build exited 0. Focused formatter check for the CI repair passed.
- **Production parity:** real daemon and ACP subprocess; mocked provider sign-in.
  Live provider OAuth account switching was not exercised.
- **PR readiness:** pending current-head CI and reviews. CodeRabbit was rate-limited;
  two authorized Opus fallback attempts failed ACP initialization before review.
  No review approval or quiet round is claimed.

## Review Corrections After the Lab

Greptile identified that startup/refusal latency was not charged to the window and
that a continuation discarded attachment bytes. The session owner now retains an
expiry from the first auth failure, caps waits and runtime startup to the remaining
time, attempts native session reload, and keeps image/PDF bytes on the continuation
when reload falls back to a new session.

The focused `TestPromptProviderAuthRecovery` passed, including deterministic
startup/refusal expiry and native-load-refusal attachment regressions. Per the
operator's host-load restriction, no local lint, race, build or new lab run was
performed for these corrections. Current-head CI owns those gates. The earlier
lab evidence above remains evidence for its recorded build, not this later patch.
