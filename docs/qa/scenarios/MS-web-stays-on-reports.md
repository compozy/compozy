---
id: MS-web-stays-on-reports
area: MS
title: A stays-on agent's reports, relayed approvals and supersede states read in its main session
persona: Dora
journey: J-31
expected: A report lands in the transcript without starting a turn. Picking a proposal sends one ordinary prompt and folds to "You picked". After an absence with enough check-ins, a While you were away card tells the night and only what needs you stays a card. Asks from child sessions dock on the main session with who asked and where, and answering settles the child's request. Expired, provider-ended and interrupted runs each say what happened and offer one ordinary prompt. A superseded session is read-only and links to the current one.
entry_points: web Sessions window; main session opened from the avatar row
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-stays-on-placement, ET-native-tool-approval-grants
---

Create a stays-on agent in a project, let a child session ask for permission, and answer it from the
main session. Then shorten `agents.stays_on.approval_timeout` and let one expire. Restart the daemon
during a held request to see the interrupted report.

QA impact 2026-11-01 (stays-on F3): proposals need the proactive slice, away reads need
`GET /api/agent-instances/{id}/away`, and the superseded state needs `instance_id` on the session
payload. Until each lands, those paths fall back to no digest and no read-only state. Focused suites
own the states. Not walked.
