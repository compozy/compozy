# BUG-20261004-native-approval-timeout-pending: Canceled native approval remains pending

- **Status:** open
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

Root cause and repair are pending. The diagnostic difference between operator CLI tool-info and
the hosted session's permission mode is not yet classified and is not asserted as this bug's cause.
