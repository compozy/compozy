# BUG-20261004-palette-approval-cannot-resume: Pending palette commands cannot be approved

- **Status:** verified
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
management at the authorized resume boundary. The repair exposes the existing coordinator through explicit HTTP/UDS and CLI decisions and
keeps a Web-initiated pending invocation open for approval. Focused API/CLI and Web suites pass;
real-service replay is still pending, so the finding remains open.

## Repair replay — 2026-10-04

The new Web approval opens under resume-editorial. Deny resolves the approval without executing;
UDS status is denied and the independent task query stays empty. The dialog nevertheless closes
before showing the terminal result, so a decision error could also disappear. The replay ends
with fail; repair dismissal before a fresh session. Evidence: palette-decision-replay-ended-summary.json
and palette-decision-replay-denial-closed.png in the cycle evidence directory.

The second replay fixes denial feedback, with independent no-effect readback. Approval now reaches
approved, then failed in the extension backend; no task is created. Web shows Command failed but
omits the error.message available through approvals show. The walk ends before engineering
diagnosis. Evidence: docs/qa/evidence/2026-10-02-untested/palette-decision-replay2-ended-summary.json;
recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/palette-decision-replay2.

## Verification — 2026-10-04

The third fresh Dora replay approves a command in Web, then resolves a second command through CLI
after switching the operator to another profile. Both create exactly one task under resume-editorial.
Foreign ownership, malformed decisions, duplicate in-flight invocations and repeated terminal
decisions are refused. Disabling the extension before approval makes the policy recheck fail without
creating its task; the dialog displays the persisted reason. UDS task reads, direct HTTP detail and a
refreshed Web deep link confirm the saved work. Denial feedback is covered by the second replay.

Evidence: palette-decision-replay3-ended-summary.json, palette-decision-replay3-approved.png,
palette-decision-replay3-unavailable.png and palette-decision-replay3-task-refreshed.png under the
cycle evidence directory. The affected gate passes Go lint, race tests and 6,927 Web tests
(palette-host-binding-delivery-gate.json). The complete profile scenario still needs its separate
pending-lifecycle, remote and authenticated-session refusal legs; this bug is verified independently.
