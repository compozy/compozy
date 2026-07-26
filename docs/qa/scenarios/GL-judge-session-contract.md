---
id: GL-judge-session-contract
area: GL
title: Constrain and clean up every Goal judge session
persona: Lea
journey: J-26
expected: Each agent command-judge attempt has a verdict-only capability boundary, captures one schema-valid verdict or one bounded typed failure, and releases its temporary session and process on success malformed output failure cancellation and replay.
entry_points: web Goal Run and agent session list; HTTP Goal turns; daemon provider lifecycle
qa_status: blocked-verify
bug_ids: BUG-20260713-goal-judge-unconstrained-leaks-session
fix_status: fixed
retest_status: pending
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/provider-attempt.json; /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/judge-attribution/
last_report: docs/qa/reports/2026-07-14-consumer-saas-growth.md
overlaps: GL-004;GL-037
---

The live Cursor/Grok run proves the product boundary: Goal work sessions may use tools, but verdict-only judge sessions must not inherit that runtime authority or survive their one criterion.

Final retest: real Cursor/Grok judge `sess-284fdef67433e103` returned one strict JSON verdict with zero tool events and stopped after approving `looprun-a6a4368bf1fc8c49`. Active-judge Clear then canceled and joined `sess-3e07f85d0d2ac987` with no surviving system session or successor generation.

QA impact 2026-07-14: judge prompts now carry exact typed role/attempt/correlation metadata used by concurrent ACP fixtures. Reset pending a final-worktree real-provider lifecycle replay.

2026-07-14 final-worktree attempt: both Claude judge sessions reached the provider but returned the typed session-limit boundary before a verdict. Deterministic ACP attribution E2E passed; real-provider promotion remains blocked until the limit resets.
