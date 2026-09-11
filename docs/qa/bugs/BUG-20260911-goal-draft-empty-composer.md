# BUG-20260911-goal-draft-empty-composer: A generated Goal draft never reaches the composer

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-26 draft an objective before activation
- **Scenarios:** GL-013; TA-104
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

In fresh real Cursor/Grok4.6 High Fast session sess-ec096d1951e83b23, submit `/goal draft Summarize kickoff.md into a short field report, verify that the workspace identity is included, and do not change any files. Draft only; do not perform the work.` The ordinary turn streams an expanded objective and clauses. After it ends, the composer is empty and there is no Draft goal command affordance. Repeat through Web without concurrent submissions using `/goal draft Read kickoff.md and report the workspace ID in one sentence. Do not change files.` The final answer appears but the composer is still empty. Public Run list stays empty and Goal status has a null snapshot. Stop reaches stopped before diagnosis.

Evidence: goal-draft-mid-full.txt; goal-draft-isolated-full.txt; goal-draft-empty-composer.png (reviewed); goal-draft-isolated-idle.json; goal-draft-runs.json; goal-draft-goal.json; goal-draft-stopped.json, all in the dated evidence directory.

## Admission observations

During the first draft, CLI with --queue and HTTP both reject with goal_draft_requires_idle while surrounding status reads show active_prompt true and queue0. A later UDS attempt arrives after that draft ends and is accepted as an ordinary 200 stream; it is not a busy-admission pass. No full GL013 race or TA095 ingress pass is claimed.

## Diagnosis and contract

The normal/draft stream intentionally passes unchanged through createGoalAwareFetch. Current useSessionChatRuntime.onFinish acknowledges recovery and invalidates reads, but has no path from the completed draft answer to composer authority. GoalComposerAffordance still declares draft and its renderer/test/story supports staging, while useSessionGoalHeader produces only replace affordances. PR271's full body was reviewed: it explicitly retains Draft goal command staging and does not retire the draft workflow. The July Use-as-Goal inert bug concerns a different, explicitly selected response action; do not reopen that record for automatic draft completion.

No fix implemented yet. Reuse the existing runtime-provider/transport suites for completed draft prefill, author-text preservation, failed/cancelled/non-draft exclusion, and zero automatic Goal submission. Use the existing composer authority, preserve line-oriented clauses, and keep stream decoding under its current owner.

## Repair in progress

The successful chat completion now recognizes the matching operator draft request and emits its final proposal through the existing session store. The same-session composer reuses its existing protected prefill, focus and persistence path. Earlier tool-planning text is excluded and clause newlines remain intact. Abort, error, disconnect and non-stop completion do not produce a draft.

Cross-surface impact: no native-tool, CLI/HTTP/UDS, hook/extension, config, persistence-schema or official-skill contract changes. Web chat/composer handoff only; emitted events carry the session identity and the original request must match its message ID. Existing draft storage and authored-text guard remain the owner. GL013/TA104 own acceptance; TA095 admission remains unchanged.

Canonical provider integration reproduced one red case, then60 tests passed including authored-text protection. Added ordinary/cancelled/incomplete exclusions are being checked with the adjacent thread/store suites. Production build, required gate and fresh same-persona replay are pending.

## Verified replay

Fresh Lea session sess-a451bde5b8a3c90b confirms Cursor/grok-4.6/high/fast at both effective runtime and ACP option. The final proposal prefills exactly one /goal with verification and constraint lines, survives reload byte-for-byte, and creates no Run/Goal. The authored-text retry records active_prompt true immediately after typing; the note remains after completion and reload. Public Goal is null, Run inventory empty, and stop reaches stopped. docs/qa/evidence/2026-09-10-qa-execution-unblock/goal-draft-retake-proof.json records reviewed screenshots and timing exclusions.

Focused provider/thread/store209 tests passed, including ordinary/cancelled/incomplete exclusions. Replaced unsupported Array.findLast with ES-target-compatible filtering after typecheck. Final root Turbo build/typecheck and make gate passed771files/7218tests, lint0warnings/0errors. TA104 is fixed-and-verified using existing exact-replacement evidence; GL013 still awaits its exact admission race.

Fix commit before subsequent rebases: 07c5a4504.
