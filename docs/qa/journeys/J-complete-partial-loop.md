# J-complete-partial-loop — Author and complete a routed partial Loop

A Loop author expresses routing and partial fan-out policy, runs it wide, and trusts every surface
to tell complete success apart from partial success.

**Adjacent-canary note (agent-comms cycle).** Loops were explicitly not supposed to change behavior
in the agent-comms change: they adopt the unified contract regime but keep their own records and
create no call records (ADR-012). What actually moved underneath them is real, though — action
validation, JSON extraction and the repair prompt left `internal/loop/action_schema.go` for
`internal/contracts`, and the task-side blanket 64 KiB result ceiling was replaced by the
`[calls.results]` budget policy. This journey is where that claim gets tested rather than assumed:
a `run-agent` node's `output_schema` must still behave exactly as it did, and an invalid payload must
still be unable to settle as succeeded.

```mermaid
flowchart TD
    A[Entry: Loop docs, visual editor, or YAML authoring] --> B[Author ask, route, review, strategy, naming, and predicate policy]
    B --> C{Validate and publish}
    C -->|lint error| D[Repair the exact field without losing draft state]
    D --> C
    C -->|valid| E[Run a wide fan-out through a bounded active window]
    E --> E1[Inspect run-agent child-session lineage]
    E1 --> SCH{run-agent node declares output_schema}
    SCH -->|payload conforms| SOK[Validated when produced AND again when it settles]
    SCH -->|payload does not conform| SBAD[Cannot settle as succeeded — the node fails with the validator's own errors]
    SCH -->|result over the effective result budget| SBIG[Fails on the budget without leaking its lease]
    SOK --> NOCALL[No call record is created anywhere — loops adopt the contract regime, not the call domain]
    SBAD --> NOCALL
    SBIG --> NOCALL
    NOCALL --> F{Settlement strategy}
    F -->|fail_fast| G[Keep completed lanes and cancel unfinished siblings]
    F -->|best_effort threshold met| H[Settle collect partial and continue downstream]
    F -->|wait_all| I[Wait for every materialized lane]
    G --> J[Read route, prune, progress, and lane-control history]
    H --> J
    I --> J
    J --> J1[Confirm settled run-agent sessions are stopped]
    J1 --> K[Fresh CLI, HTTP/UDS, native, SSE, and Web reads agree]
    K --> L[True end: terminal status and completion_state tell the same truth]
    B -.->|author closes editor| X1[Abandon: chrome and draft state restore without publishing]
    E -.->|daemon restarts mid-window| X2[Resume: no lane duplicates and active width remains bounded]
```

```yaml
journey:
  id: J-complete-partial-loop
  name: "Author and complete a routed partial Loop"
  value_statement: "A Loop author can express routing and partial fan-out policy, run it at wide width, and trust every surface to distinguish partial success from complete success."
  personas: [Bruno]
  entry_points:
    - url: "web /loops/:name/editor and /loop-runs/:runId"
      origin: in-app-nav
    - url: "CLI: compozy loop validate|create|run|status|runs and node verbs"
      origin: direct
    - url: "HTTP/UDS Loop definition, run, node-control, and event routes"
      origin: direct
    - url: "native tools in compozy__loops"
      origin: direct
  actions:
    - step: 1
      verb: "Author and validate the graph grammar"
      expected_observable: "Ask, route, review, strategy, bind_as/index_as, progress, stop_when object, and on_eval_error round-trip with deterministic lint."
    - step: 2
      verb: "Run a wide fan-out and interrupt it mid-window"
      expected_observable: "At most the declared width is active, restart does not duplicate a lane, and per-lane controls affect only the addressed item."
    - step: 3
      verb: "Let fail_fast or best_effort settle the collect"
      expected_observable: "Completed work is preserved, unfinished work is canceled by cause, oversized action results fail without leaking their lease, progress is truthful, run-agent child sessions stop at terminal settlement, and partial coverage remains first-class."
    - step: 4
      verb: "Refresh and compare terminal projections"
      expected_observable: "Run status, completion_state, collect counts, route causes, and bounded history agree across all public surfaces."
    - step: 5
      verb: "Run a run-agent node whose output_schema the payload satisfies, then one it violates, then one over the result budget"
      expected_observable: "The conforming payload is validated both when produced and when it settles; the violating one cannot settle as succeeded and reports the validator's own errors; the oversized one fails on the effective budget without leaking its lease; and no call record is created for any of them."
  goal:
    observable: "The graph reaches its declared terminal path with exact partiality, route, and lane provenance."
    side_effects: [route-decision, bounded-window-materialization, strategy-cancellation, completion-state-event]
  true_end_state: "After refresh, the editor still round-trips the definition, every run surface reports the same complete or partial outcome with no duplicate lane, and a compozy call list over the same workspace shows that the Loop created no call records while running."
  exit:
    natural: "Author lands on a terminal run whose story and Inspect data explain how the graph settled."
  abandonment:
    - at_step: 1
      how: "The author closes the editor with an unpublished draft."
      resume: "Editor chrome and draft state restore; the published definition is unchanged."
    - at_step: 2
      how: "The daemon restarts while only part of the fan-out window is materialized."
      resume: "Recovery advances the same window without exceeding width or executing a lane twice."
  crosses: [DSL-and-linter, editor-codec, coordinator-routing, fanout-window, internal-contracts-registry, config-lifecycle, CLI, HTTP, UDS, native-tools, SSE, web-run-story]
```
