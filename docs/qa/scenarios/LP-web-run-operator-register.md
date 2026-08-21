---
id: LP-web-run-operator-register
area: LP
title: Reach the run's shape, roster and history through one Inspect disclosure
persona: Dora
journey: J-diagnose-loop-run-operator
expected: "Inspect" is a single in-column disclosure — never a sheet and never a route change — holding four lanes over one read model: Graph, Nodes, Generations, Events. Graph is the default lane and renders the authored topology with per-node icon+text state chips, fan-out drawn as one entity with a rollup and one lane per worker, edge liveness toward the running node, and `pending` (reachable, dashed outline) visually and semantically distinct from `not_taken` (route evidence, neutral-dim). The lane auto-centres on whatever needs a human and says so in its foot. A node opens a panel carrying every recorded attempt (including a single one), next-retry, error class, cancellation cause and actor, plus Open session / Open record / View child run — all valid after the run ends, with a session retention has removed degrading to "Session no longer available" and a never-taken node offering no links at all. Nodes lists every node × round including healthy ones, follows the run's current round by default and shows a Round column under "All rounds"; a step that started and has not ended reads "in progress" with its elapsed clock and never "not started", and usage reads tokens beside a cost the column header labels an estimate. Generations shows per-round outcomes in plain words with no raw verdict enum, the round's own tokens and labelled estimated cost, a truthful still-running / partly-settled / interrupted state, and scores only where the loop defines them; Compare and Fork stay on every round. When the run holds more steps than one read returns, every roster-derived surface says so and offers both "Read the rest" and the exact `compozy loop nodes --run <run id> --all`. Under prefers-reduced-motion the edge pulse is unmounted, not paused.
entry_points: web /loop-runs/:id Inspect; GET /loop-runs/:id/nodes; GET /loop-runs/:id/timeline
qa_status: untested
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-web-detail-inventory-contract;LP-web-timeline-graph-rows;LP-quarantine-diagnose-requeue;LP-web-attention-loop-rows
---

story: As an operator diagnosing a run I open one disclosure and see the executing graph, locate the hot or failed node, read its attempts and error class, and jump to its session or execution record without leaving the page.

QA dependency update 2026-08-21: BUG-20260719-autonomous-progress-unobservable is verified through
a fresh public-read observer replay. This Web row remains `untested` because the focused closure did
not walk Inspect, Graph, Nodes, Generations, Events, retention, paging, or reduced-motion behavior.

steps:
1. Open a live run with a fan-out and an open approval, then open Inspect.
2. Confirm the Graph lane is default, the fan-out is one entity with a rollup, and the lane centred on the gate.
3. Confirm every state chip carries an icon and a literal word, and that a pending node reads differently from a not-taken one.
4. Click a node with two attempts; confirm the panel shows the attempt history, the failure class in plain words, and working session/record links. Repeat on a node with a single recorded attempt and confirm it is shown too.
5. Switch to Nodes and confirm healthy nodes are listed with attempts, duration and tokens beside a cost the header labels an estimate. Confirm a currently running step reads "in progress" with an elapsed clock rather than "not started".
6. Switch the round filter to "All rounds" and confirm each row names its round, so the same step in two rounds is never ambiguous.
7. Switch to Generations and confirm per-round outcomes read as sentences (no `invalid_output`-style enum), each round carries its own tokens and labelled estimated cost, an unfinished round says so, and a loop that defines no scoring shows no score. Confirm Compare and Fork are still offered.
8. Open a node whose session retention has removed and confirm the panel says "Session no longer available" instead of offering a link that 404s.
9. On a run larger than one roster read, confirm the truncation is stated on both the default read's progress panel and the Inspect foot, and that "Read the rest" loads more.
10. Re-run steps 2-3 with prefers-reduced-motion enabled and confirm the edge pulse is absent from the DOM.
