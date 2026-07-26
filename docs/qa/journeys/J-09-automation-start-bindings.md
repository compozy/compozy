# J-09 — Attach an automation start-binding to a Loop

Plug-and-play initiation (PRD F7, §9.14, ADR-007). A Loop declares which start surfaces it accepts (`start[]`); event-driven starts (schedule, webhook, trigger) ride AGH's **existing** automation primitives — a Trigger or Job pointed at the Loop — never a parallel trigger system. The operator attaches one from the Loop detail's **Start bindings** panel; a binding outside the declared `start[]` is rejected, not silently dropped.

```mermaid
flowchart TD
    A[Entry: loop-detail › Start bindings panel] --> B[Row 1: declared start[] kinds as read-only mono chips]
    B --> C[One row per attached loop-target automation: name, kind tag, enabled dot, meta cron/endpoint/event]
    C --> D{Add binding}
    D -->|Add trigger / Add schedule| E[Existing automation create sheet, pre-targeted at this Loop]
    E --> F[Target step: Run loop → loop picker + typed-input form + payload-mapping table]
    F --> G{Kind in the Loop's start[] allowlist?}
    G -->|no| H[Create-time 422; a fire would also fail with a deterministic ReasonCode — rejected, not dropped]
    G -->|yes| I[Side effect: automation created with a loop target inputs ⊕ input_mapping]
    I --> J[True end: the paged detail list shows the binding; the schedule/webhook fires a real loop_run, never the session-direct branch]
    C -.->|operator opens Add then cancels| X1[Abandon: no automation created; the panel is unchanged]
    F -.->|required input unmapped| X2[Abandon: a fire with a missing required input fails the automation run WITHOUT creating a loop_run]
```

```yaml
journey:
  id: J-09
  name: "Attach a schedule/webhook/trigger start-binding to a Loop"
  value_statement: "An operator makes a Loop run hands-free by attaching an existing automation pointed at it — bounded by the Loop's declared start surfaces."
  personas: [Marina, Bruno]
  entry_points:
    - url: "web /loops/:name (loop-detail) › Start bindings › Add trigger / Add schedule"
      origin: in-app-nav
    - url: "web automation create sheets (Target step: Run agent | Run loop)"
      origin: in-app-nav
    - url: "CLI/HTTP: automation create with a loop target (existing /api/automation/jobs|triggers)"
      origin: direct
  actions:
    - step: 1
      verb: "Open the Start bindings panel"
      expected_observable: "Declared start[] kinds render as read-only chips; each attached loop-target automation shows name, kind tag, enabled dot, and a meta line (cron+next fire / endpoint / event)"
    - step: 2
      verb: "Add a trigger or schedule pre-targeted at the Loop"
      expected_observable: "The existing automation create sheet opens pre-targeted; the Target step offers Run loop with a loop picker, typed-input form, and payload-mapping table"
    - step: 3
      verb: "Complete and save the binding"
      expected_observable: "A kind in the Loop's start[] saves an automation with a loop target; a kind NOT in start[] returns a create-time 422"
  goal:
    observable: "The Loop gains a working start-binding: the schedule/webhook fires a real loop_run (never the session-direct branch), inputs resolved as static ⊕ input_mapping"
    side_effects: [automation-created, loop-target-binding]
  true_end_state: "The automation appears in the independently paged Start bindings detail under the loop=<name> filter; the catalog makes no sampled binding claim; on fire it produces a loop_run and records the automation_runs row as delegated with loop_run_id set."
  exit:
    natural: "Operator returns to the loop detail with a live start-binding attached."
  abandonment:
    - at_step: 3
      how: "Operator attaches a binding whose kind is not in the Loop's start[] allowlist."
      resume: "Create-time 422 (and a fire-time ReasonCode) — the binding is rejected, not silently dropped."
    - at_step: 2
      how: "A required input is left unmapped in the payload-mapping table."
      resume: "A fire with a missing required input fails the automation run WITHOUT creating a loop_run."
  crosses: [loop-detail-start-bindings, automation-triggers/jobs, ADR-007-loop-target, workspace-scoping, automation_runs]

design_reference:
  screens:
    - "docs/design/opendesign/loop-detail.html (LOOPS-DESIGN-SPEC §4.2 — Start bindings panel; §9.14)"
  truthful_ui_checks:
    - "The declared start[] allowlist is enforced: a binding outside the declaration is rejected (create-time 422 + fire-time ReasonCode), never silently dropped (PRD F7)."
    - "A schedule/webhook fires a real loop_run via the loop-target branch, never the session-direct branch (ADR-007)."
    - "Start bindings page independently in Loop detail under the canonical loop=<name> filter; the catalog exposes no sampled binding badge until an exact aggregate exists."
    - "A workspace-scoped automation cannot target a Loop in a different workspace (a global automation names its target workspace explicitly)."

e2e_backbone:
  runtime:
    - "E2E-runtime-5: start a loop from schedule/webhook/trigger (via loop-target automations) and reach an identical terminal outcome (ADR-007)."
  web:
    - "E2E-web-17: render every page of the Start bindings panel (declared start[] + attached automations), open trigger/job create sheets pre-targeted, complete the Target step (Run loop + typed inputs + payload mapping), surface the create-time 422 for a missing start[] kind, and prove the catalog makes no sampled binding claim."
  integration:
    - "Integration-31: start a loop from a loop-target automation end-to-end (webhook + schedule) → loop_run (never session-direct), automation_runs delegated with loop_run_id, reject undeclared kind (422 + ReasonCode), reject a cross-workspace target, expose bindings via the loop=<name> filter."
  followups:
    - "AB-001 — the loop e2e-web seed harness must seed enough attached automations to exercise continuation in the Start bindings panel and the catalog's no-sampled-badge contract in Playwright."
```
