# J-recover-loop-node-failure — Author, run, repair, and finish a Loop

A Loop author declares the failure contract, starts real work, and hands a failed or interrupted
node to an operator who can understand and repair it without losing healthy work. The journey ends
only when the authored recovery path and its effects survive a fresh cross-surface read.

```mermaid
flowchart TD
    A[Entry: Web catalog/editor or structured CLI/API] --> B[Author lifecycle policy, effects, and required inputs]
    B --> C[Validate, publish, and start one real Loop run]
    C --> D{What happens while the node is live?}
    D -->|classified failure| E{Recovery precedence}
    D -->|pause or managed-session death| F[Park or resume one fenced continuation]
    D -->|cancel| G[Commit canceled(operator_cancel), fence new work, and stop owned sessions]
    E -->|retry eligible| H[Durable backoff and same-generation retry]
    E -->|error route| I[Skip success-only work and run fallback]
    E -->|unhandled or repeated| J[Create repair context or quarantine the lane]
    J --> K[Operator diagnoses, repairs, and requeues or resumes]
    F --> L[Continue healthy sibling work]
    H --> L
    I --> L
    K --> L
    L --> M[Deliver only committed node and terminal effects]
    G --> M
    M --> N[Run reaches its truthful terminal outcome]
    N --> O[Fresh Web, CLI, and HTTP reads agree]
    O --> P[True end: author and operator can explain what ran, recovered, and finished]
    H -.->|client closes or daemon restarts during backoff| X1[Abandon: durable schedule remains]
    X1 -.->|operator returns| O
    J -.->|operator leaves before repair| X2[Abandon: lane stays parked while independent work remains truthful]
    X2 -.->|operator returns and repairs| K
```

```yaml
journey:
  id: J-recover-loop-node-failure
  name: "Author, run, repair, and finish a Loop"
  value_statement: "A Loop author and operator can take real work from declared failure policy through a truthful repair and terminal outcome without duplicate or hidden work."
  personas: [Lea, Bruno]
  entry_points:
    - url: "web /loops, /loops/:name/editor, /loops/:name/run, /loop-runs/:id"
      origin: in-app-nav
    - url: "CLI: compozy loop validate|run|runs show|nodes|waits"
      origin: direct
    - url: "HTTP/UDS Loop definition, run, lifecycle control, inventory, and event routes"
      origin: direct
  actions:
    - step: 1
      verb: "Author and validate retry, error-route, wait, and effect policy"
      expected_observable: "The editor and daemon expose the same accepted definition, diagnostics, and effective lifecycle envelope"
    - step: 2
      verb: "Start the Loop with its declared inputs"
      expected_observable: "The catalog, run form, structured response, and run detail identify the same run"
    - step: 3
      verb: "Trigger a classified failure or interrupt one live node"
      expected_observable: "The runtime takes one retry, route, repair, pause, resume, or forced Cancel authority with durable provenance"
    - step: 4
      verb: "Diagnose and repair the affected lane"
      expected_observable: "Healthy siblings continue while the operator can inspect and requeue or resume only the actionable node; the continuation preserves the exact generation, node id, and item index while advancing attempt and epoch"
    - step: 5
      verb: "Return after the run settles and compare public reads"
      expected_observable: "Web, CLI, HTTP, UDS, SSE, and effect results expose one terminal outcome with its original classified context"
  goal:
    observable: "The Loop follows one authored recovery path, finishes truthfully, and keeps independent work intact"
    side_effects: [attempt-recorded, retry-scheduled, fallback-activated, repair-generation-started, lifecycle-effect-delivered]
  true_end_state: "After refresh or daemon restart, Web, CLI, and HTTP reads agree on the chosen recovery path, terminal cause, effect result, and node provenance with no duplicate work."
  exit:
    natural: "The author and operator can explain why the Loop continued, what was repaired, and how it finished."
  abandonment:
    - at_step: 3
      how: "The operator closes the client or restarts the daemon while a retry or wait is parked."
      resume: "The durable schedule survives; returning later shows one settled attempt and no duplicate run."
    - at_step: 4
      how: "The operator leaves a quarantined lane before repairing it."
      resume: "The lane remains visibly parked and can be requeued later while unrelated work stays intact."
  crosses: [loop-dsl, loop-coordinator, scheduler, globaldb, effects, CLI, Web, HTTP, UDS, native-tools, SSE]
```

Taxonomy note: the complete value flow covers journey and functional behavior; failure, death,
pause, quarantine, and cancel branches cover edge and recovery behavior; the browser charters own
usability, accessibility, and perceived-performance observations; daemon restart and fresh
cross-surface reads cover continuity, consistency, workspace isolation, and regression. Mobile is
deliberately covered by the separate approval journey because the editor canvas is desktop-only.
