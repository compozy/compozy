---
id: RT-call-profile-scope-isolation
area: RT
title: Keep calls and messages inside their owning profile and workspace
persona: Dora
journey: J-contain-and-audit-delegation
expected: Every call and message read, write and stream filters by profile, scope and workspace at the store layer, cross-boundary targets are denied before any side effect, aggregate reads carry owner labels without authorizing mutation, and Global scope works with no workspace.
entry_points: compozy call ses_foreign "cross-scope attempt" and compozy message send ses_foreign "cross-scope attempt"; compozy call list --all-profiles --limit 25; global-scope compozy call reviewer "global work" with no workspace; HTTP and UDS GET /api/workspaces/ws_main/calls?profile=default and GET /api/workspaces/ws_main/messages?session=ses_01JBD8G2MZTX&profile=default; compozy__agent_call with {"session_id":"ses_foreign","prompt":"cross-scope attempt"}; compozy__agent_message with {"to":"ses_foreign","text":"cross-scope attempt"}; web /agents/activity after switching profiles
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-call-golden-path; NB-agent-call-publish; ET-workspace-access-prompt-outcomes
---

Calls are profile-owned work roots, so the isolation question has three axes, not one: profile,
scope, and workspace.

Create calls and messages in two profiles and two workspaces, then try to reach across every seam —
CLI, HTTP, UDS, native tool and the web — in both directions. A cross-workspace target must fail
`call_workspace_denied` **before any side effect**, and a cross-profile typed call must be denied
too; neither may raise a permission prompt, because delegation is not a consent seam and no
permission mode or cached session answer can authorize it.

Then check the deliberate exceptions rather than assuming there are none. An explicit aggregate read
(`--all-profiles`) must return rows carrying owner labels and must authorize no mutation through
that view. Global scope with no workspace must work. And Network publish keeps the established
profile-blind delivery exception — confirm it is the only one.

Finally prove the filter lives at the store layer, not in a handler: switch profile in the web and
confirm the query cache cannot serve the previous profile's calls; address a foreign profile's call
by id and confirm it is not-found rather than forbidden-with-a-hint; and confirm a foreign-scope row
never appears in a list, a detail read, a count, an SSE-derived projection or an attention badge.
