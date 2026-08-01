---
id: LP-ratchet-climb-restore
area: LP
title: Improve, restore, and inspect the best Loop generation
persona: Bruno
journey: J-improve-loop-with-feedback
expected: A validated metric Loop promotes only approved finite improvements, restores a rejected regression from the accepted best with typed prior diagnosis, retains best when exhausted or stalled, and reports the same score and provenance on every public surface after restart.
entry_points: docs /docs/loops/ratchet and /docs/loops/dsl-reference; official Compozy skill Loop reference; compozy loop validate; compozy loop inspect; compozy loop list; compozy loop run; compozy loop status; compozy loop runs; HTTP/UDS Loop routes; compozy__loop_status; compozy__loop_runs; loop.gate.post and loop.generation.pre hooks; Loop SSE; web /loops catalog, /loops/runs outcomes, and /loop-runs/:run_id detail; extension scorer fixture
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/same-run-parity-v6.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/sse-events-v6.txt; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/loop-run-v6-full.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/official-e2e-results.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps:
---

Derived from the metric branch of `J-improve-loop-with-feedback`. Exercise a strict climb with
`stop_when` generations, a regression that restores an older best, a no-baseline rejection, and an
exhausted or stalled exit. The extension criterion must supply the same structured numeric `score`
contract as command and judge scorers. Detail surfaces own verdict/provenance; summary surfaces own
only best fields.
