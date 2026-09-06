# BUG-20260906-compaction-repeat-boundaries: Stopped and interleaved turns block later compaction

- **Status:** fixed — owning regression and repeated public/browser compaction passed
- **Impact:** Task-Blocked
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-11 / J-14
- **Scenarios:** RT-session-context-rebuild; ET-web-session-transcript-calm-grammar
- **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

After the first successful archive, the same Incident archive ledger begins at sequence4535 with session.stop_escalated followed by the confirmed session_stopped receipt. A new pressure usage update never advances generation1. The candidate selector recognizes only ACP done/error as terminal. It also groups only adjacent equal turn IDs, so a clarification interleaved before the conversation's done event blocks that complete turn.

The selector now recognizes confirmed session_stopped and computes each turn's complete boundary in one linear pass. It advances the contiguous cut only after every encountered turn's last event. Incomplete turns, missing identities and the triggering active turn remain barriers; crossing the active turn cannot archive a partial earlier turn.

The existing TestPressureCompactionArchivesCoveredReplaySpans owns stopped-suffix, interleaved-clarification and active-boundary coverage. The first two failed before the repair; all selected compaction race cases passed3.656s. Logs: .cache/sessions-qa-compaction-repeat-{red,green}.log. The test-shape heuristic flags the unchanged top-level TestContextCompactionDispatchesHooksAndUsesPatchedParams; the added cases use the required named parallel subtests.

Real re-walk on the rebuilt daemon: the open query payload needle initially has one match at generation1. Public prompt msg_find_invalidation_fixed triggers generation2; Web keeps its query and focus, reports No matches, and retains the new authored message and active assistant. The public search agrees. Two hundred events from the previously active prefix compare byte-for-byte in their authoritative content after archival. Evidence in the integrated lab: find-invalidation-{before,after}-{transcript,search}.json, find-invalidation-retained-events.json, find-invalidation-audit.json, navigation-find-invalidation-{before,after}.json/.png. Root inspected the after screenshot. No ledger content, public shape or configuration was rewritten.
