---
id: ET-web-session-context-sidebar
area: ET
title: One Context sidebar explains usage and delivered context
persona: Rafa
journey: J-14
expected: The tab-less Context sidebar shows context window, Compozy context, tokens and cost, turns, and activity in order; Compozy context and Turns ship folded behind a chevron head that carries their count. The meter shows numbers and tiers only, with a chip just for a caveat state. The three bounded tiers fit the window while raw estimates stay visible. Ownership, receipt facts, unavailable state, and observed agent-compaction markers are explicit, with a Compact now action row in the meter section for supporting agents; no CompozyOS compaction or threshold is shown.
entry_points: OS session window Context sidebar; session-context-agent and session-context-unknown acpmock fixtures; SessionInspector stories
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: ET-web-session-context-meter; RT-052; RT-060; RT-session-cost-provenance
---

1. At 1440px open Context; at 1280px verify its drawer and Escape. Verify five sections and no tabs.
2. Inspect the normal, estimate-exceeds, and over-capacity bar. Its Compozy tier is `min(injected, used, size)`; free space is never negative. Raw totals remain in the legend.
3. Expand Compozy context. Inspect full and unchanged rows, sent/last-seen turns, hook replacement, binary attachment names/bytes without tokens, startup-opaque inclusion, and rows marked `may have been summarized` after the agent reports a drop.
4. Verify unknown-with-rows has a list without a bar. Interrupt a usage read after success: last numbers stay with an unavailable chip. Fresh unknown/unavailable states do not invent counts. If a later unavailable context read carries newer token/cache aggregates, those totals update while only the previous context snapshot is retained.
5. End the stream with a usage refresh still debounced, then reopen Context. Aggregate and per-turn queries must both refresh; a token-only delta must not trigger either usage query. Compare cache and cost cells with the aggregate endpoint and provenance; compare usage-only, counter-only, delivery-only, and combined turns against `/usage/turns`. Expand Turns (its head reads the turn count). Verify the observed compaction marker text per status (in progress, completed, failed, cancelled, vendor value): "Agent compaction" (the agent compacted on its own) or "Requested compaction" (Compact now, `compozy session compact`, a tool call, or a Goal), then the status, with tokens before → after only once the next usage report is known (just the before figure, or nothing, when unknown). Rows recorded before the upgrade under Memory or Workspace knowledge render by their raw key. Reveal earlier rows after 50.
6. Compare Activity with the live working timer, child signals, transcript summary, queue, goal and runtime warning. Absent selectors produce no row; stopped session retains only available facts.
7. Open Vault separately and confirm the session-scoped secret; confirm the transcript changed-files roll-up and public ledger remain reachable in their own homes.
8. In the meter section, Compact now sits below the meter as an action row (button plus inline refusal) on a session whose agent advertises a compaction command, and the section still reads meter first; its behavior is walked in ET-session-compact-now step 8.

Final execution and Visual Contract bundles belong to task_06 VC-06–11. Implementation fixtures and focused tests are not live QA evidence.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

2026-09-15 quiet-context pass: the meter lost its `reported` chip and the `as of turn <id> · HH:MM:SS · Compaction runs at 85%` line (chip only for loading, unavailable, stale, near compaction, estimated size; the threshold stays as the bar tick). Turns now ships folded under the same chevron head as Compozy context, with `N turns` as its meta; the list, compaction markers, and `Show earlier turns` are unchanged inside the fold. Steps 1 and 5 updated. Owning unit suite and the focused browser E2E (`session context E2E-001`, five sections still in order) re-verified; the session-context design boards still draw the meter line and an always-open turn card and are superseded on those points.

QA impact 2026-10-07 (memory removal): the meter has no threshold tick or warning chip, compaction markers describe observed agent compactions only, and new turns never attribute Memory or Workspace knowledge bytes. Stale verdict reset to untested; historical evidence preserved; no QA session ran. Update 2026-10-08: the Web markers and the Compact now action row shipped (steps 5 and 8).
