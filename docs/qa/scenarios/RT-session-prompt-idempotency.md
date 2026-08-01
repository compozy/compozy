---
id: RT-session-prompt-idempotency
area: RT
title: Retry one session prompt without repeating its effect
persona: Théo
journey: J-11
expected: Repeating the same prompt or steer with the original message_id and idempotency_key returns the stored result without a second stream, queue entry, hook chain, provider call, Goal mutation, or authored transcript row; changing either bound request returns a structured conflict, and an uncertain dispatch is never resent automatically.
entry_points: Web session thread; HTTP and UDS prompt/steer routes; compozy session prompt CLI; compozy__session_prompt; Extension Host sessions/prompt
qa_status: pass
bug_ids: BUG-20260713-session-user-message-reorders-or-disappears; BUG-20260801-web-prompt-missing-message-id
fix_status: fixed
retest_status: pass
fix_commits: a73b6587; b9da1778
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/structured-replay.json; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/identity-conflicts.json; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/goal-http-replay.json; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-coderabbit-20260801-093002-740482-lab/qa-artifacts/qa/goal-replay-coderabbit.json; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-coderabbit-20260801-093002-740482-lab/qa-artifacts/qa/screenshots/coderabbit-goal-replay-clean.png; /Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/boundary-verification.json
last_report: docs/qa/reports/2026-08-01-session-extension-boundary.md
overlaps: RT-session-message-reload; RT-session-lifecycle-affordances
---

Use one real provider session and preserve the original identities for every retry. Prove the first admission and its exact replay from Web plus at least one structured agent-manageable surface. Compare receipt fields, provider-visible effects, durable event ids, queue state, and the cold-reloaded transcript. Reuse of an idempotency key with changed text/runtime/mode and reuse of a message id with a different key must return their distinct 409 diagnostics. Force or inspect a dispatch-committed receipt without retrying it; the only acceptable automated behavior is `prompt_dispatch_indeterminate`.

Planning impact 2026-07-31: new cross-surface at-most-once admission and exact message reconciliation require a fresh isolated walk before completion.

2026-08-01 isolated walk: Web admitted `message_id=we0Y9M6vnuXWdXhv` with `idempotency_key=386891c0-fb0b-4531-bb46-54c148617984` and completed turn `turn-f93e70b2d3d9ee2d`. Identical retries through HTTP and CLI returned the stored turn with `replayed=true` and no stream. Divergent idempotency-key and message-identity reuse returned their distinct `409` diagnostics. The independent transcript read still contained one authored event and one provider response, and a cold browser reload still rendered one row for each.

2026-08-01 post-review first attempt: the daemon served its embedded release Web bundle instead of the current repository build, so the new API correctly rejected the stale client's request with `message_id is required`. `BUG-20260801-web-prompt-missing-message-id` is invalid as a product finding. The scenario remains untested pending a fresh lab using the supported `COMPOZY_WEB_DIST_DIR` override.

2026-08-01 post-review production-parity retest: fresh Web assets admitted `message_id=PbuNvwcsgcxqtLXo` and completed `turn-3157dac135eba00d` exactly once. A separate CLI prompt using explicit durable identities completed `turn-ae0a591ae48f3fef`; its exact retry returned `replayed=true` and the same turn without another stream. Conflicting key and message reuse returned `prompt_idempotency_conflict` and `prompt_message_identity_conflict`. A Goal error returned HTTP 404 on both first admission and replay while changing only `replayed=false` to `true`, and a successful Goal replay preserved its run and CLI identity fields.

2026-08-01 CodeRabbit remediation reset: replayed non-OK Goal responses now remain on the failure-rendering path, explicit Web identity pairs reject partial/blank values, and transport recreation retains the shared idempotency map without reading refs during render. A fresh isolated walk is required before completion.

2026-08-01 CodeRabbit remediation retest: an exact Goal retry preserved the original identities and `goal_not_active` HTTP 404 while changing only `replayed=false` to `replayed=true`. The current compiled Web transport rendered one `/goal status` row and the actionable failure guidance, with no synthetic success. The deterministic screenshot passed visual inspection, and teardown stopped both registered processes with `clean=true` and no survivors.

2026-08-01 boundary retest: after a logical session's first provider bind, the exact CLI retry preserved
`msg-qa-sandbox-first-bind-1` and `turn-abd6e5c31301aee8` with `replayed=true`; an independent HTTP
transcript read still contained one authored message and one provider response.
