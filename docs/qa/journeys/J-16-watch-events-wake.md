# J-16 — Watch events: author, park, wake, and recover a daemon-internal watch loop

The `loops-refac` WS1 identity case (TechSpec §WS1, ADR-003/ADR-004). A loop declares an `events:` envelope of watch-events subscriptions (`{kind, filter}` CEL per entry) and, when it reaches that node, **parks** into the live, zero-cost `watching` state. A durable daemon event whose kind matches a subscription rings a typed **doorbell**: the observer evaluates the subscription's CEL against `event`+`inputs`, and a match **coalesces an idempotent wake** into the loop coordinator, which runs one round over the batch replayed from the **ledger cursor** and drives the downstream node to a truthful terminal (or re-parks if still watching).

This is **distinct from J-08**: J-08 is the extension-provided `loop.watch_source` (ADR-016 — poll/push, has a silence window that ends `stalled`). Watch-events is **daemon-internal** (ADR-003, a closed source class, **not** extension-pluggable) — the wake is event-edged off durable ledger anchors, there is **no polling and no silence-stall concept**, and durability across downtime is guaranteed by **boot reconcile + a scheduler backstop gap-check** rather than a live lease. The supported kinds phase in per ADR-004: phase A `task.*`/`loop.*`, phase B `automation.*`/`network.*`, phase C `coordinator.*`/`event.post_record` — the web kind select and the docs matrix derive from the family registry (never a hand-authored list), and pre-state/no-anchor kinds (`*.pre_*`, `network.peer.*`) lint unsupported.

```mermaid
flowchart TD
    A[Entry: author a loop with an events: watch-events node] --> B{Kind + CEL filter valid?}
    B -->|unsupported kind / too-broad filter| B2[Shared linter blocks Publish: watch_events_kind_unsupported / watch_events_filter_too_broad]
    B2 --> A
    B -->|valid, published| C[Run the loop CLI/web/native]
    C --> D[Node parks: run enters watching — live, zero-cost, subscription index hydrated on the park boundary]
    D --> E{A durable event of a subscribed kind commits?}
    E -->|no event yet| D
    E -->|non-matching or cross-workspace event| E2[Doorbell ignores it — no wake; workspace equality enforced inv7]
    E2 --> D
    E -->|matching: CEL(event,inputs) true| F[Coalesced idempotent wake → loop coordinator round]
    F --> G[Side effect: round consumes the batch from the ledger cursor → downstream node runs]
    G --> H{Still watching?}
    H -->|more to watch| D
    H -->|work done| I[True end: truthful terminal done — never coerced from exhausted/stalled]
    D -.->|daemon restarts while parked| X1[Recover: boot reconcile + backstop gap-check replay from the ledger cursor — wake EXACTLY once, never a claim]
    X1 --> F
    D -.->|no matching event ever arrives| X2[Abandon: stays dormant at zero cost — watch-events has NO silence-stall; operator stop → failed operator_stop, never coerced]
```

```yaml
journey:
  id: J-16
  name: "Author, park, wake, and recover a daemon-internal watch-events loop"
  value_statement: "An operator or agent arms a loop to sleep at zero cost until a specific daemon event happens, wakes it deterministically on a match, and trusts it survives a restart without missing or double-firing."
  personas: [Bruno, Ada]
  entry_points:
    - url: "web /loops/:name/editor (loop-editor: Watch events source + subscription form) › Publish › Run › web run-detail (park read-model)"
      origin: in-app-nav
    - url: "CLI: agh loop run  +  agh loop runs show -o json (park read-model: subscriptions, cursors, last_wake_at)"
      origin: direct
    - url: "native agh__loop_* (author/validate/run/observe entirely through structured surfaces)"
      origin: external-share
  actions:
    - step: 1
      verb: "Author a watch-events node (pick a kind from the supported matrix, write a CEL filter)"
      expected_observable: "The kind select lists exactly the registry-supported kinds; an unsupported kind or a too-broad filter (e.g. event.post_record without a session_id constraint) is rejected by the shared Go linter, disabling Publish"
    - step: 2
      verb: "Run the loop; it reaches the watch-events node"
      expected_observable: "The run enters the live watching state at zero cost; run-detail (web + agh loop runs show -o json) exposes the parked read-model — subscriptions {kind, filter}, cursors, last_wake_at — and renders nothing when absent"
    - step: 3
      verb: "A matching durable event commits (a task completes, an automation finishes, a network/coordinator/session event lands)"
      expected_observable: "The doorbell evaluates CEL(event, inputs); a match coalesces one wake and runs a coordinator round over the ledger batch; a non-matching or cross-workspace event never wakes the loop"
    - step: 4
      verb: "Repeat until the work is done or restart the daemon while parked"
      expected_observable: "Work done → truthful terminal done; a restart replays from the ledger cursor (boot reconcile + backstop) and wakes exactly once — no missed events, no double-wake"
  goal:
    observable: "The loop wakes only on genuine matches, runs its downstream work, and concludes on a truthful terminal appropriate to what actually happened"
    side_effects: [coalesced-coordinator-wake, ledger-cursor-advance, downstream-node-run, observability-events(matched/wake_enqueued/wake_error)]
  true_end_state: "A matched event drives one round to a verified done; a restart mid-park recovers and wakes exactly once from the durable ledger; content from session records (event.post_record) never crosses into loop outputs/SSE/web — each verifiable on reload and via structured status."
  exit:
    natural: "Operator/agent lands on the truthful terminal run, or the loop re-parks at zero cost awaiting the next event."
  abandonment:
    - at_step: 3
      how: "No event of a subscribed kind ever arrives."
      resume: "The run stays dormant at zero cost — watch-events has NO silence-stall (distinct from J-08); the operator stops it to failed(operator_stop) if no longer wanted."
    - at_step: 2
      how: "The daemon is restarted while the run is parked and events commit during downtime."
      resume: "Boot reconcile scans watching runs for ledger-cursor gaps and the scheduler backstop gap-check catches any missed index entry; the run wakes exactly once, never claims."
  crosses: [loop-coordinator, task/automation/network/coordinator/session event families, observe-ledger, boot-reconcile, scheduler-backstop, session-policy-gate(for downstream run-agent nodes), workspace-isolation]

design_reference:
  screens:
    - "docs/design/opendesign/loop-editor.html (LOOPS-DESIGN-SPEC §4.6 — Watch events source entry + subscription list editor)"
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4 — parked read-model: subscriptions/cursors/last_wake_at)"
  truthful_ui_checks:
    - "watching is a LIVE zero-cost dormant state — not terminal, not running (ADR-013/ADR-003); no lease is held while parked."
    - "the park read-model renders ONLY when the run has watch-events nodes; the absent block renders nothing (no fabricated dormant state); CLI/HTTP/UDS/native parity."
    - "the kind select derives from the family registry matrix per phase — never a hand-authored TS enum (AGH-65 lesson); the lint text names the supported set so an agent self-corrects without docs."
    - "the wake is event-edged (doorbell/ledger) — there is NO poll cadence and NO silence-stall knob; a silent source is dormant, not stalled."
    - "workspace equality is enforced at the doorbell (invariant 7) — a cross-workspace event never matches, wakes, or appears in the read-model."
    - "event.post_record requires a session_id filter and excludes record content — only record_type/sequence/session correlation cross into loop outputs/SSE/web (redaction reviewed)."

e2e_backbone:
  runtime:
    - "task 11 E2E-runtime: parked loop with pinned task.status_changed + task.run.completed subscriptions; unrelated task completes under the watched parent → wake → downstream acpmock run → terminal done."
    - "task 11 E2E-runtime crash fail-injection: events written 'during downtime' → boot reconcile finds the co-durable row and reserves the coordinator run on boot."
    - "task 13 integration: automation.run.completed wake + network work-persisted wake (boot-reconcile variants)."
    - "task 14 integration: coordinator.stopped wake; session-scoped event.post_record wake with redaction + cross-session non-match; doorbell hot-path benchmark."
  web:
    - "task 12 E2E-web / component: codec round-trips a watch-events node (kind + CEL); run-detail renders subscriptions/cursors/last_wake_at from a fixture, absent block renders nothing; agh-ui-screenshot capture cited."
  followups:
    - "AB-009 — a real-daemon watch-events browser seed (park a run, commit a matching event, drive editor→park→wake in Playwright). Task-11 ships the runtime lane and task-12 ships codec/component + screenshots; the live-daemon Playwright walk for LP-043/LP-044 rides AB-009 (same gap class as AB-001 for J-01)."
    - "Phase gating — LP-047/048 (phase B) and LP-049/050 (phase C) only exist once tasks 13/14 land; qa-execution walks the phase-A rows (LP-040..044) first and treats later-phase rows as blocked-until-implemented if a phase is unshipped at run time (record the skip, do not invent a pass)."
```
