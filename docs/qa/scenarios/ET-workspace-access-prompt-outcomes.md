---
id: ET-workspace-access-prompt-outcomes
area: ET
title: Resolve a cross-workspace prompt and expire its session answer
persona: Bruno
journey: J-cross-workspace-access
expected: An approve-reads session hitting the native-tool boundary raises one pending permission offering allow_once, allow_session, reject_once, and reject_session; once answers apply to that call only, session answers apply to every seam for the rest of the session, and stopping the session clears the answer so the next crossing prompts again.
entry_points: compozy__workspace_info; compozy__task_run_claim_next; compozy spawn --workspace; compozy session approve <session-id> --request-id <request-id> --decision <allow-once|allow-always|reject-once|reject-always>; POST /api/workspaces/:workspace_id/sessions/:session_id/approve; compozy logs --type workspace.access_granted; GET /api/logs; compozy__logs; compozy__observe_search; /docs/sessions/permissions#the-prompt-in-approve-reads
qa_status: blocked-verify
bug_ids: BUG-20260730-tool-invoke-202-empty-success
fix_status: fixed
retest_status: blocked — HTTP 202 decoding is fixed, but the complete once/session/reject/expiry matrix needs a dedicated approval harness
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/notes/cross-workspace-access-results.md
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-workspace-access-mode-matrix; ET-native-tool-approval-grants
---

Start an `approve-reads` session in workspace A and call a native tool naming workspace B. Confirm one
pending permission appears with the four daemon-computed options `allow_once`, `allow_session`,
`reject_once`, and `reject_session`, labeled Allow once, Allow for this session, Reject once, and
Reject for this session.

Answer `allow-once` and confirm the call proceeds and the next crossing prompts again. Answer
`reject-once` and confirm the call is denied with the permission-mode hint while a later crossing
still prompts.

Answer `allow-always` and confirm it resolves to `allow_session`: later crossings by that session
succeed with no prompt, and crossings at the task, spawn, and coordination seams — which never prompt
— now succeed too. Confirm no approval record appears in any list or revoke surface: the answer is
daemon memory only. Stop the session, start a new one for the same agent, and confirm the first
crossing prompts again. Repeat the expiry check across a daemon restart.

Answer `reject-always` and confirm it resolves to `reject_session` and denies every later crossing by
that session at every seam.

Leave one prompt unanswered past `[tools.policy] approval_timeout_seconds` and confirm the request
denies and stores no session answer. Inspect `compozy logs`: the initial prompt-eligible policy miss
is `workspace.access_denied`, while later evaluations admitted by `allow_session` are
`workspace.access_granted`. Confirm target, seam, source, and mode remain attributable.

Answer through both operator surfaces — `compozy session approve <session-id> --request-id
<request-id> --decision <allow-once|allow-always|reject-once|reject-always>` and `POST
/api/workspaces/:workspace_id/sessions/:session_id/approve` — and confirm the same four option ids
and the same resulting behavior. Read the audit trail through all four reader surfaces (`compozy logs
--type`, `GET /api/logs`, `compozy__logs`, `compozy__observe_search`) and confirm they agree on the
same decisions. Audit appends are best effort, so a decision missing from a degraded event store is a
store finding, not proof the decision differed — verify against a healthy store.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-007). Planning flag
only; no QA replay ran in this documentation slice.

Planning 2026-07-29 (task 06): re-homed to `J-cross-workspace-access` and re-assigned from Ada to
Bruno — the agent triggers the prompt, but the answering, the consent lifetime, and the audit read
are all operator work. Entry points widened to the cross-seam reuse surfaces, both answer surfaces,
and all four audit readers. Settled by charter `CH-cross-workspace-consent-audit`.

QA 2026-07-29: allow/reject once and session answers passed through both operator answer surfaces.
Session grants crossed task, coordination, and spawn seams, expired on stop and daemon restart, and
an unanswered request timed out without storing consent. CLI, HTTP, native logs, and native search
agreed on the attributable audit trail.
