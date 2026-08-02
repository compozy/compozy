# J-configure-and-run-loop — Reuse Loop settings from a file

A delivery builder keeps repeatable Loop limits in a reviewed JSON or YAML file. They preview the
effective run settings, persist the same settings for later runs, and confirm an invalid field is
rejected without changing the saved configuration.

```mermaid
flowchart TD
    A[Entry: documented Loop config file] --> B{Choose JSON or YAML}
    B --> C[Preview a run with --config-file]
    C --> D{Known fields?}
    D -->|yes| E[Inspect effective config in structured output]
    D -->|no| F[Read the validation error and keep prior config]
    E --> G[Persist the file with loop configure --file]
    G --> H[Read the configured Loop again]
    H --> I[True end: preview and persisted values match the file]
    C -.->|operator stops after preview| X[Abandon: no persisted config changes]
```

```yaml
journey:
  id: J-configure-and-run-loop
  name: "Configure and run a Loop from reusable files"
  value_statement: "A builder can reuse reviewed Loop limits in JSON or YAML without retyping flags or losing strict validation."
  personas: [Bruno]
  entry_points:
    - url: "CLI: compozy loop run --config-file"
      origin: direct
    - url: "CLI: compozy loop configure --file"
      origin: direct
  actions:
    - step: 1
      verb: "Preview a Loop run with a JSON or YAML config file"
      expected_observable: "Structured output contains the requested snake_case fields, including nested enabled checks"
    - step: 2
      verb: "Persist the same file as the Loop configuration"
      expected_observable: "A fresh structured read returns the same effective settings"
    - step: 3
      verb: "Try one unknown field"
      expected_observable: "The CLI rejects the file before mutation and the previous configuration remains intact"
  goal:
    observable: "Previewed and persisted Loop settings match the reusable file"
    side_effects: [loop-config-updated]
  true_end_state: "A fresh CLI read shows the requested nested and scalar settings, while invalid input leaves them unchanged."
  exit:
    natural: "The builder can run the Loop again without reconstructing the override flags."
  abandonment:
    - at_step: 1
      how: "The builder stops after the preview."
      resume: "No workspace configuration changed; the same file can be used later."
  crosses: [CLI, config-lifecycle, JSON, YAML]
```
