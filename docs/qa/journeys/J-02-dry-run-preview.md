# J-02 — Dry-run: preview the first round before committing

Before committing a run, the user resolves global/workspace/definition/run input values, validates
the effective per-task runtime, and sees the first generation's plan **without spending budget,
creating a run, or spawning ACP** (PRD "Previewing", ADR-017 §9.1). The value is confidence: "here
is exactly what will run" with zero side effects and inspectable value origins.

```mermaid
flowchart TD
    A[Entry: run form, config path, or structured run surface] --> B[Resolve run > workspace > global > definition input defaults]
    B --> B2[Apply optional runtime rules and overrides]
    B2 --> C{Dry run / validate}
    C -->|inputs invalid / required missing| D[Validation error inline; NO plan rendered; nothing created]
    D --> B
    C -->|inputs valid| E[Call DryRun: validate against schema + contract]
    E --> E2{Runtime and input-default validation}
    E2 -->|unknown key, wrong type, unknown model, missing secret| D
    E2 -->|valid| F[Return PlanPreview: values + origins + applied runtime intent + gen-1 plan + contract]
    F --> G[Side-effect check: NO loop_run row, NO task_run, ZERO budget spent]
    G --> H[True end: preview toast/panel shows the gen-1 plan; run form still ready to Run for real]
    H -.->|user reads plan and leaves| X1[Abandon: no run leaked — a dry-run that created a row would be the bug]
    H -->|looks right → Run loop| I[Proceeds into J-01 (real run)]
```

```yaml
journey:
  id: J-02
  name: "Dry-run a Loop to preview the first round without side effects"
  value_statement: "A user sees the first generation's plan and confirms their inputs are valid before spending any budget or creating a run."
  personas: [Lea, Bruno]
  entry_points:
    - url: "web /loops/:name/run › Dry run (loop-run-form)"
      origin: in-app-nav
    - url: "CLI: compozy loop run --name software-delivery --input slug=x --dry-run"
      origin: direct
    - url: "native tool: compozy__loop_run with dry=true"
      origin: in-app-nav
    - url: "CLI/native config plus HTTP/UDS input-default management for loops.inputs.<loop>.<key>"
      origin: direct
  actions:
    - step: 1
      verb: "Fill the declared inputs"
      expected_observable: "Run values override workspace, global, then definition defaults per key; explicit false, zero, and valid empty values remain present"
    - step: 2
      verb: "Press Dry run"
      expected_observable: "Inputs and effective runtime validate before ACP spawn; input_default/runtime_validation errors are typed and deterministic, with NO plan or persisted run"
    - step: 3
      verb: "Read the returned plan"
      expected_observable: "A PlanPreview with resolved input values and origins, effective runtime intent, the generation-1 materialized node list, and the contract"
  goal:
    observable: "The gen-1 plan is shown (toast + preview) and the run form remains armed to Run for real"
    side_effects: []
  true_end_state: "CLI, HTTP, UDS, and native reads agree on resolved values, origins, and validation diagnostics; no loop_run, task_run, or ACP session was created and no budget was spent."
  exit:
    natural: "User either presses Run to execute (→ J-01) or leaves; either way no run exists from the dry-run."
  abandonment:
    - at_step: 2
      how: "Dry-run surfaces a validation error the user can't resolve; they leave."
      resume: "Nothing created; re-entering the form re-validates cleanly."
    - at_step: 3
      how: "User previews, decides not to run, closes the form."
      resume: "No run leaked; the absence of a Runs-list row is the confirming observable."
  crosses: [config.toml, CLI, HTTP, UDS, native-config-tools, run-form-schema, input-default-resolver, runtime-validator, loop.Service.DryRun, ACP-spawn-boundary, budget-accounting]

design_reference:
  screens:
    - "docs/design/opendesign/loop-run-form.html (LOOPS-DESIGN-SPEC §4.3; ADR-017 §9.1)"
  truthful_ui_checks:
    - "Dry-run creates NO loop_run and spends NO budget — a run appearing in the Runs list after a dry-run is a truthful-UI violation."
    - "No Cost cap (USD) input in the form the dry-run validates (cost is display-only)."
    - "The preview reflects the CANONICAL resolved config (defaults ⊕ config.toml ⊕ loop_config ⊕ per-run overrides, clamped), not the raw definition defaults."
    - "Explicit false, zero, and valid empty defaults retain presence and origin; absence is never inferred from a zero value."
    - "Unknown input keys, wrong types, unknown providers, and missing bound secrets fail before ACP spawn with stable typed diagnostics; exact model IDs for known providers pass through unchanged."

e2e_backbone:
  runtime: []
  web:
    - "E2E-web-2: '... return a plan on Dry run ...' (run-form dry-run path)."
  integration:
    - "Integration-8: Should validate inputs and return a gen-1 plan on dry-run against a real resolved definition with no loop_run created (ADR-017)."
  unit:
    - "Unit-29: Should return a PlanPreview from DryRun and write no loop_run/task_run and spend no budget."
    - "Unit-27/28: effective-config merge + ceiling clamp feeding the preview."
  followups:
    - "AB-001 — dry-run preview render on the real run form is part of the loop e2e-web seed-harness gap; behavior asserted at integration + component until the Playwright seed lands."
    - "LP-loop-input-defaults — exercise sequential config writes and cross-surface value/origin parity."
    - "LP-runtime-validation-preflight — distinguish static definition validation from effective run validation and prove no ACP spawn."
```
