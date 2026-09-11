# BUG-20260911-goal-draft-empty-composer: A generated Goal draft never reaches the composer

- **Status:** open
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
