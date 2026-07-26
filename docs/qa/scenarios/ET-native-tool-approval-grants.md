---
id: ET-native-tool-approval-grants
area: ET
title: Set, remember, and revoke native-tool approval decisions
persona: Théo
journey: J-answer-agent-requests
expected: Allow-always and reject-always decisions survive daemon restart only for the exact workspace, agent, tool, and input digest; explicit agent-wide and tool-wide decisions set through Web, CLI, HTTP, UDS, and native tools survive restart without an input digest; every surface lists the same rows; revocation removes each decision everywhere and wider allows never exceed the configured tool-policy ceiling.
entry_points: Native-tool permission prompt; Web Settings / General; agh tool approvals set/list/revoke; PUT/GET/DELETE /api/tool-approval-grants; agh__tool_approvals_set/list/revoke
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-037;ET-038
---

Trigger native-tool approvals for allow and reject decisions from two agents and with distinct
inputs in one workspace. Restart the daemon and confirm only an exact workspace, agent, tool, and
input-digest match is reused without a second prompt. Set one agent-wide decision and one tool-wide
decision through each management surface, restart, and verify the wider keys contain no input
digest. Confirm another workspace sees no rows. Compare Web, CLI, HTTP, UDS, and native-tool list
output, revoke each row through a different management surface, then confirm the next matching
invocation prompts again. Under every tool mode, verify wider allows remain below the policy ceiling
and that one-shot approval tokens, ACP subprocess permissions, and sandbox policy remain unchanged.

QA impact 2026-07-15: new durable workspace-scoped native-tool approval decisions, management
surfaces, and Web revoke flow. Planning flag only; no QA session ran in this implementation slice.

QA impact 2026-07-19: explicit agent-wide and tool-wide creation now spans native, CLI, HTTP, UDS,
and Web surfaces. Reset to `untested`; this is a planning flag, not a retest result.

Phase C planning 2026-07-19: linked to J-answer-agent-requests; settles US-001 (D1, ADR-001).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Grant row before and after daemon restart via `agh__tool_approvals_list`, CLI `-o json`, and HTTP
  parity (identical data).
- Explicit agent-wide and tool-wide set requests (native, CLI, HTTP, UDS, Web) with their persisted
  no-digest keys.
- Zero-prompt transcript for a matching call, a prompt for a non-matching agent, and the
  revoke → re-prompt transcript.
- A deny-all run proving a stored allow grant never overrides the mode ceiling, and a grant-store
  read-error run falling to the prompt (never auto-approve).

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-001-durable-approval-grants-for-native-tools
