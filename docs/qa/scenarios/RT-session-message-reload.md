---
id: RT-session-message-reload
area: RT
title: Authored session messages survive reconciliation and reload
persona: Théo
journey: J-11
expected: Ordinary prompts and structured slash commands render exactly once in authoritative server chronology before the work they initiate; live SSE reconciliation and a cold permalink reload preserve the exact authored text without duplication, movement, or loss.
entry_points: web session thread; POST session prompt; transcript REST and SSE
qa_status: pass
bug_ids: BUG-20260713-session-user-message-reorders-or-disappears; BUG-20260801-web-prompt-missing-message-id
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38; a73b6587
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/session-prompt-reloaded-centered.png; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/session-prompt-reloaded-deterministic.png; /Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/cold-reload-dom-counts.json
last_report: docs/qa/reports/2026-07-31-session-prompt-idempotency-post-review.md
overlaps: RT-045; RT-058; TA-089
---

story: As a returning user I can trust that every message I authored remains in its original place when live agent output arrives and when I reload the session later.

The 2026-07-13 live replay first reproduced duplicate/reordered ordinary prompts and a `/goal` command that disappeared after reload. The same-persona post-fix replay used a fresh Cursor/Grok 4.5 session, completed two ordinary turns plus a two-turn approved Goal, then reloaded the exact permalink. All three authored inputs remained present exactly once and in strict request/response chronology.

2026-07-14 explicit retest after the final daemon rebuild: the same permalink reloaded in 874 ms. Both ordinary prompts and the exact `/goal` input remained present exactly once and kept their original order (`orderPreserved=true`, `allExactlyOnce=true`).

QA impact 2026-07-14: the Automation + Hermes rebase combined client message identity with typed ACP event payloads. Historical evidence is retained, but live reconciliation and cold reload must be replayed from the final worktree.

2026-07-14 final-worktree control: the exact authored message remained in fresh transcript/history reads before deletion. The complete Web E2E gate independently passed session reload chronology. Retest promoted to pass.

QA impact 2026-07-31: prompt admission now preserves Assistant UI's authored `message_id` as the durable transcript row id, reconciles only by exact id, suppresses provider-authored echo chunks, and returns exact retries without a second stream or effect. Historical evidence remains valid for the prior fix, but live settlement and a cold permalink reload must be walked again from this worktree before restoring `pass`.

2026-08-01 isolated retest: a real Codex session rendered the authored prompt and `QA-OK` exactly once after live settlement. Reloading the canonical permalink produced one matching user row and one matching assistant row. The independent CLI history read retained one durable `user_message` with the original Assistant UI `message_id`, sequence 3, ahead of the only agent response at sequence 6.

2026-08-01 post-review first attempt: the daemon served its embedded release Web bundle instead of the current repository build, so the current API rejected the stale client's request before provider dispatch. The environment finding is invalid; live settlement and reload remain untested pending a fresh lab using `COMPOZY_WEB_DIST_DIR`.

2026-08-01 post-review production-parity retest: current `web/dist` rendered the Web-authored prompt and `ONE-ROW-OK` once after live settlement. The same canonical permalink was cold-reloaded after an independent structured prompt and exact retry. DOM counts remained exactly one for both authored prompts and both assistant responses, and the durable history retained the Web message at sequence 3 and the CLI message at sequence 13 under distinct stable turn ids.
