---
id: LP-web-attention-loop-rows
area: LP
title: Attention bell surfaces loop-node rows only when the daemon reports them
persona: Dora
journey: J-05
expected: The OS attention bell shows "Loop nodes waiting on you" / "Loop nodes needing attention" rows only when a workspace-scoped limit-1 probe of the loop-nodes route returns at least one item in that state; with nothing parked the rows are absent (absence is the signal). The rows carry the shared state glyphs, no counts, add no badge to the bell trigger, and deep-link to `/loop-runs?nodes=waiting` / `?nodes=attention`.
entry_points: web OS shell attention bell
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: f1e91fc5
evidence:
last_report:
overlaps: LP-web-detail-inventory-contract
---

Added by the loops visual-contract parity pass (2026-08-14). Walk needs a workspace with and without parked loop nodes; deferred to the next seeded QA cycle — `attention-model.test.ts`, `attention-bell.test.tsx`, and `use-os-attention.test.tsx` cover both probe states at 9a694ff2.
