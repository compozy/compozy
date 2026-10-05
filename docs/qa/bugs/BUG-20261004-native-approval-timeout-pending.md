# BUG-20261004-native-approval-timeout-pending: Canceled native approval remains pending

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona:** Dora · **Journey:** J-operate-profiles
- **Scenarios:** ET-profile-approval-owner-resume; RT-session-attention-catalog
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

A real hosted session in approval-guides requests compozy__session_stop after its outer provider
command is allowed. The separate native permission starts at 11:54:02 UTC and times out at
11:56:02 UTC; its structured tool result reports approval_required and approval_canceled. The
driver missed that 120-second window, so the refusal itself is expected and supplies no successful
owner-resume verdict. The target session remains active.

After the provider turn completes at 11:57:33 UTC, the same permission still appears pending in
CLI session status, CLI interactions and independent UDS interactions. The session badge remains
waiting-for-auth. Responding with its exact request and turn IDs fails with pending permission
not found. The durable/public interaction projection therefore advertises a request that is no
longer actionable. The walk ends before engineering inspection, and its owned sessions are stopped.

Session: sess-9a8f4d529c42bffa; interaction: int_01M43C75JBGQEAX7P9GEN3FM3J;
request: abf89009-57bd-4475-8930-8fada2dad1b2; turn: turn-ef77824daac8776e.
Owner profile: 01M43BW15NP8J12E19NA1QSFTJ (approval-guides).

Evidence under docs/qa/evidence/2026-10-02-untested/: profile-approval-resume-after-native-allow-transcript.json,
profile-approval-resume-after-native-allow-operator.json, profile-approval-resume-native-owner-allow.json,
profile-approval-resume-stale-interaction-uds.json and profile-approval-resume-walk-ended.json.

The diagnostic difference between operator CLI tool-info and
the hosted session's permission mode is not yet classified and is not asserted as this bug's cause.

## Root cause confirmed

The ACP interactive permission handler emits the pending event, then removes its in-memory request
on return. Its connection-close and caller-cancellation branches return the canceled ACP outcome
without a terminal permission event. The native tool approval bridge cancels that caller context
after 120 seconds. Session attention therefore never receives a terminal transition; status and
interaction reads remain pending until restart recovery later expires the record. The main lab's
normal restart cleared the stale badge, confirming recovery but not repairing live cancellation.

The owning repair must emit the cancellation from the ACP permission owner and carry it through
the existing durable interaction transition. Operator approval choices remain distinct from
runtime cancellation. The existing ACP permission bounds/connection-death suite owns event closure;
the existing session attention/transition suite owns persisted terminal status. The bridge also
currently classifies an ACP canceled response after its own deadline as generic approval_canceled;
its timeout suite should retain the actual timeout reason.

## Repair

The ACP owner now emits the terminal system cancellation on caller/connection termination; session
projection persists the existing canceled status, and the bridge retains the timeout cause.
The existing ACP permission, session catalog transition and bridge error suites failed before the
repair and pass together with the race detector afterward. Evidence:
`native-approval-cancellation-owning-{red,green}.json` and
`native-approval-test-shape-baseline-comparison.json`. The test-shape helper's remaining findings
are unchanged from HEAD. The fresh real daemon replay below and the enclosing affected delivery gate pass.


## Verification — 2026-10-04

Dora's fresh native request `a7275b1e-1135-4d5c-8f60-0604cb2bb354` remains unanswered. It starts
at 13:13:54.660347Z and emits system cancellation at 13:15:54.661646Z. CLI and independent UDS
interaction reads become empty, the badge leaves waiting-for-auth and the bridge result retains
approval_timed_out. The owned target remains active. A late allow returns already-resolved with
resolved_decision=canceled and cannot execute that expired call. Daemon PID 91398 remains unchanged.

A separate fresh operator session receives a timely allow for request
e86562f8-c670-47dd-b8bb-1d43adf27e4a. The native tool returns verified cooperative stop; independent
UDS confirms the exact target stopped with its original profile ID. No pending interaction remains.
The provider turns finish and both operator sessions are stopped through the CLI. Final daemon
status reports zero active sessions. Neither source inspection nor editing occurs during the walk.

Evidence: native-cancel-replay-*.json, especially ended, late-decision, timeout-transcript,
canary-native-allowed, canary-target-uds and cleanup-daemon. The native permission fix is verified;
the separate durable palette approval defect still prevents the complete owner-resume scenario.
Fix commit: 9f1457296. The enclosing delivery gate passes codegen, zero-issue Go lint,
race-enabled Go tests and all 6,921 Web tests, lint and typecheck. Receipt:
native-cancel-settings-restart-delivery-gate-scoped.json.
