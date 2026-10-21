---
id: LP-web-attention-loop-rows
area: LP
title: Attention bell surfaces loop-node rows only when the daemon reports them
persona: Dora
journey: J-05
expected: The OS attention bell shows "Loop nodes waiting on you" / "Loop nodes needing attention" rows only when a workspace-scoped limit-1 probe of the loop-nodes route returns at least one item in that state; with nothing parked the rows are absent (absence is the signal). The rows carry the shared state glyphs, no counts, add no badge to the bell trigger, and deep-link to `/loop-runs?nodes=waiting` / `?nodes=attention`.
entry_points: web OS shell attention bell
qa_status: untested
bug_ids:
fix_status:
retest_status: pending
fix_commits: f1e91fc5
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-web-detail-inventory-contract
---

Added by the loops visual-contract parity pass (2026-08-14). Walk needs a workspace with and without parked loop nodes; deferred to the next seeded QA cycle — `attention-model.test.ts`, `attention-bell.test.tsx`, and `use-os-attention.test.tsx` cover both probe states at 9a694ff2.


### All-projects aggregate integration (A3)

The bell now reads Loop requests and node inventory across registered projects plus User,
with an explicit all-profiles lens. Create pending requests and waiting/attention nodes in two
projects, in User, and in a second profile. Verify each occurrence appears once, retains its
owner binding, and an empty workspace is labeled `User`. Request-backed waits must not duplicate
request notifications. Terminal input inventory still uses the existing project fan-out until A4.

Backend evidence: `docs/qa/reports/2026-10-21-all-projects-a3.md`. The integrated browser re-walk
is pending UW/A4; the previous report above is historical evidence, not a verdict for this change.
