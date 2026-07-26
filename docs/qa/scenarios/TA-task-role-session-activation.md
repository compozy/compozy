---
id: TA-task-role-session-activation
area: TA
title: Activate a task-role worker after starvation recovery
persona: Bruno
journey: J-complete-task-tree
expected: A spawned task-role session receives one initial turn after ACP readiness, claims and attaches its assigned queued run through the lease contract, and surfaces a prompt/start failure promptly instead of idling until TTL.
entry_points: scheduler starvation recovery; Web agent Sessions; Web task run detail
qa_status: untested
bug_ids: BUG-20260713-task-role-session-never-starts;BUG-20260713-task-role-dispatch-repeats;BUG-20260713-cursor-agent-mode-unavailable
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-session-never-starts.dom.txt; /var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/sessions/ws_06366aad69887872/sess-fbc0f0f9edf012ea/ledger.jsonl; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-synthetic-turn-fixed-agent-mode-blocked.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-repeated-synthetic-turns.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/agh71-faithful-child-b-one-run.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: TA-024;TA-025;TA-026;TA-parent-rollup-completion
---

The task-role prompt must be actively dispatched; storing it only as `creation_profile.prompt_overlay` does not activate the worker. Verify one-turn idempotency and typed failure recovery as well as the happy path.

2026-07-13 fixed-path retest: live session `sess-1e9a13013651c8b0` received the exact correlated first turn and responded in 21 seconds. The scenario remains blocked before claim by BUG-20260713-cursor-agent-mode-unavailable because Cursor reports Ask mode for this system session.

2026-07-13 residual: the same session later contained 14 assistant responses for the same still-queued run with no user prompt or explicit recovery. The first-turn fix is therefore not durable-idempotent after prompt completion; BUG-20260713-task-role-dispatch-repeats tracks the provider-spend/transcript-flood regression.

2026-07-14 final retest: four fresh Cursor/Grok task-role sessions each received one correlated activation, claimed one existing run, and completed exactly once. The faithful AGH-71 children retained one session/run pair each and no polling redispatch appeared.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
