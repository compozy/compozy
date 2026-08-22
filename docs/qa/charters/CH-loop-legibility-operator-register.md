# CH-loop-legibility-operator-register: Interrupt a run and prove Inspect still tells the truth

```yaml
charter:
  id: CH-loop-legibility-operator-register
  mission: "As Dora, break a live run — kill a node, quarantine a lane, prune a session, restart the daemon mid-diagnosis — and prove the one Inspect disclosure keeps naming the right node, the right cause and the right actor across all four lanes, with pending never conflated with never-taken."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-diagnose-loop-run-operator
  scenarios: [LP-web-run-operator-register, LP-web-timeline-graph-rows, LP-web-detail-inventory-contract, LP-web-attention-loop-rows]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Confirm Inspect is one in-column disclosure with four lanes over one read model — the URL must not change and no sheet may appear — with Graph as the default lane, auto-centred on whatever needs a human and saying so in its foot; then confirm a reachable node with no output row (pending, dashed outline) reads visually and semantically different from one that lost its route (not_taken, neutral-dim), and that the never-taken node offers no links at all."
      - "Open a node with two attempts and one with exactly one: both must show full attempt history, next retry, error class in plain words, and the cancellation cause and actor, with links still working after the run ends. Prune a node's session and confirm the panel degrades to Session no longer available rather than offering a link that 404s."
      - "Kill the daemon while a node panel is open; after boot confirm the sweep has already settled terminal leftovers, the graph reflects converged state, and no ghost live record survives. Then park and unpark a node and confirm the attention bell's loop rows appear only when the workspace probe returns items — absence is the signal — and deep-link to the filtered inventory."
      - "Sweep the remaining lanes and the accessibility floor: Nodes lists healthy nodes, a started-but-unfinished step reads in progress with an elapsed clock and never not started, cost is labelled an estimate, and under All rounds every row names its round; Generations reads per-round outcomes as sentences with no raw verdict enum, says so when a round is unfinished, shows scores only where the loop defines them, and keeps Compare and Fork on every round; a run larger than one roster read states its truncation on both the progress panel and the Inspect foot and offers the exact compozy loop nodes --run <run-id> --all; and under prefers-reduced-motion the edge pulse is unmounted from the DOM rather than paused, while every state chip pairs an icon with a literal word and the disclosure and node selection stay reachable by keyboard."
    must_avoid:
      - "Judging node identity by a hard-coded round; rows are located by their node id inside the roster."
      - "Accepting an in-memory projection after an intervention — re-read from CLI or HTTP before calling it converged."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier — the operator half of the two-register hot spot. Owns Safety Invariant 14 (the
source-to-projection mapping is total, and `not_taken` projects only from durable route evidence)
against ADR-003, which reversed the graph-eng deferral and brought a run-bound live DAG into scope
specifically because `BUG-20260719-autonomous-progress-unobservable` proved an operator could not
distinguish progress from a stall. The Interrupt Tour matches the register's purpose: this surface
exists for the moment something has already gone wrong, so the session must arrive by breaking
things. The node inventory and attention-bell scenarios ride along as the S7/attention blast radius
of the same redesign — both were reset by task_05's vocabulary and routing changes.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
