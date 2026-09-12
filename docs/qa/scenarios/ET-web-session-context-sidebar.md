---
id: ET-web-session-context-sidebar
area: ET
title: One Context sidebar explains usage and delivered context
persona: Rafa
journey: J-14
expected: The tab-less Context sidebar shows context window, Compozy context, tokens and cost, turns, and activity in order. The three bounded tiers fit the window while raw estimates stay visible. Ownership, receipt facts, unavailable state, and compaction archive facts are explicit; no agent compaction success is inferred.
entry_points: OS session window Context sidebar; session-context-agent and session-context-unknown acpmock fixtures; SessionInspector stories
qa_status: pass
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
4. Verify unknown-with-rows has a list without a bar. Interrupt a usage read after success: last numbers stay with an unavailable chip. Fresh unknown/unavailable states do not invent counts.
5. Compare cache and cost cells with the aggregate endpoint and provenance; compare usage-only, counter-only, delivery-only, and combined turns against `/usage/turns`. Verify archived and not-archived compaction marker text. Reveal earlier rows after 50.
6. Compare Activity with the live working timer, child signals, transcript summary, queue, goal and runtime warning. Absent selectors produce no row; stopped session retains only available facts.
7. Open Vault separately and confirm the session-scoped secret; confirm the transcript changed-files roll-up and public ledger remain reachable in their own homes.

Final execution and Visual Contract bundles belong to task_06 VC-06–11. Implementation fixtures and focused tests are not live QA evidence.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.
