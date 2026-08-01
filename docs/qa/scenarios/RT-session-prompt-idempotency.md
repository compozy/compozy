---
id: RT-session-prompt-idempotency
area: RT
title: Retry one session prompt without repeating its effect
persona: Théo
journey: J-11
expected: Repeating the same prompt or steer with the original message_id and idempotency_key returns the stored result without a second stream, queue entry, hook chain, provider call, Goal mutation, or authored transcript row; changing either bound request returns a structured conflict, and an uncertain dispatch is never resent automatically.
entry_points: Web session thread; HTTP and UDS prompt/steer routes; compozy session prompt CLI; compozy__session_prompt; Extension Host sessions/prompt
qa_status: untested
bug_ids: BUG-20260713-session-user-message-reorders-or-disappears
fix_status: in-progress
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-message-reload; RT-session-lifecycle-affordances
---

Use one real provider session and preserve the original identities for every retry. Prove the first admission and its exact replay from Web plus at least one structured agent-manageable surface. Compare receipt fields, provider-visible effects, durable event ids, queue state, and the cold-reloaded transcript. Reuse of an idempotency key with changed text/runtime/mode and reuse of a message id with a different key must return their distinct 409 diagnostics. Force or inspect a dispatch-committed receipt without retrying it; the only acceptable automated behavior is `prompt_dispatch_indeterminate`.

Planning impact 2026-07-31: new cross-surface at-most-once admission and exact message reconciliation require a fresh isolated walk before completion.
