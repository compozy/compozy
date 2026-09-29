---
id: RT-provider-error-handoff
area: RT
title: A rate-limited or unauthenticated turn on a user session prescribes handoff
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: When a session/prompt on a user session fails as rate_limited or not_authenticated, the persisted error event carries provider_error.next_action "handoff" with guidance "continue this session with another agent or route: compozy session continue <id> --agent <name>", the transcript marker is recorded, the session stays active, and nothing is created automatically; the same failures on spawned, coordinator, or system sessions keep retry/login, and pre-acceptance refusals keep use_fallback.
entry_points: compozy session prompt <id> "…"; compozy session events <id> --type error -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/events; compozy session continue <id> --agent <name>; compozy session list -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-cli-session-continue; RT-session-fallback-chain
---

Planning 2026-09-28 (session-continue-fork task_03): new behavior. Walk with an acpmock provider scripted
to rate-limit `session/prompt` (or a real provider out of quota):

1. Create a user session and prompt it until the provider rate-limits the turn.
2. `compozy session events <id> --type error -o json`: the error's `provider_error` has
   `code: provider_rate_limited`, `next_action: handoff`, and guidance naming
   `compozy session continue <id> --agent <name>`.
3. The session stays `active`; `compozy session list` shows no new session.
4. Follow the guidance: `compozy session continue <id> --agent <other>` works as in
   `ET-cli-session-continue`.
5. Repeat step 1 with a spawned child session: `next_action` stays `retry` (or `login` for
   `provider_auth_required`).

Automated evidence at authoring time: the session-owner decoration unit cases and the prompt contract
case for a user session. task_07/08 own the walk.

Planning 2026-09-28 (session-continue-fork task_07): task_08 adds an acpmock step that fails
`session/prompt` with a JSON-RPC error whose text the classifier maps to `rate_limited` (for example
"429 rate limit exceeded") and a second variant for `not_authenticated`, then un-fixmes E2E-004 in
`web/e2e/__tests__/session-derive.spec.ts` and walks steps 1–5 against that fixture. Step 5's
spawned-child leg uses the same fixture agent under a spawning parent. Plan: `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`.

Automated evidence 2026-09-28 (session-continue-fork task_08, part A; not a walk verdict): acpmock
gained the `fail_prompt` driver_control action (`error_message`, optional `error_code`, default
-32603). `internal/testutil/acpmock/testdata/provider_error_fixture.json` agent `handoff-agent` answers
`rate limit this turn` with "429 rate limit exceeded" and `auth lapse this turn` with "401
unauthorized: authentication required". `TestDriverFailPromptClassifiesProviderError` proves the two
turns classify as `provider_rate_limited` and `provider_auth_required` through the real ACP driver, and
web E2E-004 proves the user-session marker carries `data-provider-next-action="handoff"`. Use this
agent for steps 1–5 of the walk.
