# BUG-20260906-compaction-startup-hooks: Startup hook events block pressure compaction

- **Status:** fixed — pressure/archive/browser re-walk passed
- **Impact:** Task-Blocked
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-11 context recovery
- **Scenarios:** RT-session-context-rebuild
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

The live Incident archive receives valid ACP usage_update used90/size100 with pressure threshold0.85 and enabled compaction. No session.compaction_fired event appears. The actual ledger begins with hook.dispatch.start and hook.dispatch.complete for session.post_create, each with its own synthetic turn ID. completePriorTurnPrefix incorrectly requires these standalone hook audit groups to contain an ACP terminal event, so every subsequent complete conversation turn remains outside its candidate span.

The repair classifies the two canonical hook dispatch audit types alongside existing standalone markers and clarification receipts. Unknown events and incomplete conversation turns remain barriers. The existing pressure compaction suite covers the startup-hook prefix without weakening its incomplete-turn case.

Evidence: integrated lab evidence/navigation-prefix-events.json and navigation-compaction-events.json; source usage event sequence4563, active turn turn-74a1ecda00c987c8. Regression and public re-walk pending.

Canonical regression failed before the change and selected compaction race suites passed3.720s (.cache/sessions-qa-compaction-hooks-green.log). Rebuilt daemon then admitted span1..4534. The controlled summary fixture initially matched a broad earlier turn and lacked four mandatory headings; those fixture mistakes were corrected without weakening production validation. The configured built-in summary role completed; materialized generation advanced0→1, all4585raw events remain and806pre-existing events compare exactly. Both active authored message msg_history_compaction_final and assistant turn-2a47a312263fa523 remain in the public transcript and the real browser. Reconnect with epoch0/generation0 produces generation_mismatch; a cursor100 in generation1 produces cursor_expired; both snapshots contain active identity. Evidence: integrated lab navigation-compaction-audit.json, navigation-compacted-events-all.json, navigation-compacted-transcript.json, navigation-generation-reset.sse, navigation-expired-cursor.sse, navigation-compaction-web-verified.json/.png, project/.compozy/memory/project_checkpoint_summary.md.
