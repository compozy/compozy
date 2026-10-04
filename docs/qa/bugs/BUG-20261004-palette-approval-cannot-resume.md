# BUG-20261004-palette-approval-cannot-resume: Pending palette commands cannot be approved

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, resolve a destructive command after switching profiles
- **Scenarios:** ET-profile-approval-owner-resume
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In CH-profile-archive-race-guards / Multi-Tab Tour, create empty owned profiles resume-editorial
and resume-drafts. Select resume-editorial in the attached Web client. Invoke `profile.delete`
through `compozy cmd-palette invoke`, supplying that client, the Studio Operations workspace,
`--profile resume-editorial`, and `--arg profile=resume-drafts`.

The command returns `approval_pending` with approval apr_bc140827-498f-4f42-bbdd-673060d9fd53.
Owner-selected `approvals show` reads the pending record. Archive/delete plans list the approval
as a blocker and work counts remain zero. No resolution control appears in the Web client.
The public CLI offers only `approvals show` and `approvals cancel`; the public OpenAPI likewise
offers status and cancellation, with no decision endpoint. The two-minute approval expires
without running the action. The profile selection is restored to editorial.

## Evidence

Cycle evidence prefix `docs/qa/evidence/2026-10-02-untested/profile-owner-palette-resume-`:
`ids.json`, `pending-ui.json`, `approval-help.json`, `invoke-help.json`, `expired-show.json`,
`ended-ui.json`, and `walk-ended.json`. The exact browser recording is
`/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/profile-owner-palette-resume`.
This attempt does not prove owner-preserving resume. Native session permission approvals are a
separate mechanism and do not close this durable palette approval.

## Engineering diagnosis after the walk

The asynchronous approval coordinator implements approval and denial internally, but production
callers expose only status, cancellation, and expiry. No production caller selects its approved
outcome. This differs from the earlier owner-filtering bug: inspection/cancellation now work,
while approval remains unreachable. Restore the public operator decision path through the
existing coordinator and recheck profile ownership, availability, session binding, and local
management at the authorized resume boundary. No implementation change has been made yet.
