# J-31: Steward an agent definition

```mermaid
flowchart TD
  A[Entry: agent detail deep link] --> B[Inspect profile and focused editors]
  B --> C[Edit What it does or Check-ins]
  C --> D{Digest current?}
  D -->|yes| E[Save and re-read]
  D -->|409| F[Reload, reapply, retry]
  E --> G[Try a dry run, then run a real check]
  G --> H[Open Model or What it can use and save]
  H --> I[Fresh reload confirms daemon truth]
  C -.-> X[Abandon with dirty editor]
  X --> Y[Guard keeps editing or discards explicitly]
  Y --> C
  I --> J[True end: detail reflects persisted definition]
```

```yaml
journey:
  id: J-31
  name: Steward an agent definition
  value_statement: "An operator can inspect and safely edit definition-owned state without losing concurrent work."
  personas: [Bruno, Sol]
  entry_points:
    - url: /agents/$name
      origin: direct
  actions:
    - step: 1
      verb: Inspect profile, places, and authored files
      expected_observable: Origin, runtime, configuration, sessions, diagnostics, and file state match daemon reads
    - step: 2
      verb: Edit focused pages, Memory, and authored context
      expected_observable: CAS, validation, permission, and unsaved states are explicit and keyboard reachable
  goal:
    observable: Fresh reads show the saved definition and authored files
    side_effects: [definition-updated, authored-context-updated, optional-instance-check]
  true_end_state: Reloaded detail and structured reads agree on digest, origin, and values
  exit:
    natural: Agent overview
  abandonment:
    - at_step: 2
      how: Navigate away with unsaved changes
      resume: Guard offers Keep editing or explicit discard
  crosses: [web, agent-api, authored-context-api, sessions-api]
```
