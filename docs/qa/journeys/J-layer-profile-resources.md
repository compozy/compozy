# J-layer-profile-resources — Give one context its own agents, settings, and keys without forking anything

An operator drops resources and settings into a profile layer, commits a per-profile folder to a
repository for the team, and points one provider at a different key — then checks which layer
actually won. Nothing is copied, nothing personal is written into the repository, and a write that
lands under a layer something else overrides says so instead of claiming success.

```mermaid
flowchart TD
  A1[Entry: drop a folder under ~/.compozy/profiles/<p>/ or <ws>/.compozy/profiles/<p>/] --> B[Next catalog read composes four additive layers]
  A2[Entry: compozy config set / config get / agent list / skill list] --> B
  A3[Entry: Settings Persona, Hooks, Command palette, Memory pages] --> B
  A4[Entry: compozy secret set|rm and provider inspect] --> S1
  A5[Entry: agent memory read or native config tool] --> B
  B --> C{Layer named a profile that exists?}
  C -->|no| C1[Dormant: content does not apply and a diagnostic names the path and the create action]
  C -->|yes| D[Most specific layer wins; the winner and what it shadowed are both inspectable]
  C1 --> C2[Create or rename to that name] --> D
  D --> E{Write a config key}
  E -->|no explicit scope| E1[Target the layer that owns the context: user file under default, profile file otherwise]
  E -->|--scope user or workspace| E2[Target that layer instead]
  E -->|daemon-identity key on a profile layer| E3[Refuse profile_config_key_denied with allowed prefixes; no file and no apply record change]
  E1 --> F
  E2 --> F{A more specific layer already sets this key?}
  F -->|yes| F1[Saved but not applied — the response names the winning layer]
  F -->|no| F2[Saved and applied]
  F1 --> G
  F2 --> G[Side effect: one apply record naming the layer and path; MCP sidecars merge in their layer's slot]
  E3 --> G
  S1 --> S2{Secret reference form}
  S2 -->|process environment for a profile secret| S3[Refuse profile_secret_env_forbidden and point at the vault path]
  S2 -->|vault| S4[Stored under the owning profile's prefix; containment refuses anything outside it]
  S4 --> S5[Provider inspect names the source: profile override or user default; native logins say machine-level plainly]
  S5 --> S6{Override removed?}
  S6 -->|yes| S7[Acknowledged removal; new work falls back to the user credential]
  S6 -->|no| H
  S7 --> H
  S3 --> H
  G --> H[Read memory in the profile tier]
  H --> I{Memory maintenance move still pending?}
  I -->|yes| I1[Profile-tier reads refuse fail-closed rather than reading the old path]
  I -->|no| I2[Profile-tier entries stay inside their owner; the aggregate is refused for memory]
  I1 --> J
  I2 --> J[Rename the profile]
  J --> K[Side effect: machine folders and vault refs rewrite in the rename transaction; repo folders are offered as pending edits; extension placements go dormant]
  K --> Z[True end: effective config, resource shadow order, credential source, MCP entries, and memory tier all agree across CLI, HTTP, UDS, Web, and native reads, and repository files are byte-identical unless the operator accepted and committed a rename]
  E -.->|operator abandons the write mid-flow| X1[Abandon: no partial file and no apply record; the previous good value stands]
  C1 -.->|operator ignores the dormant hint| X2[Resume: nothing nags; the content wakes the moment the name exists]
```

```yaml
journey:
  id: J-layer-profile-resources
  name: "Give one context its own agents, settings, and keys without forking anything"
  value_statement: "I can specialise one context's agents, settings, MCP servers, memory, and provider keys, share the repository half with my team, and always see which layer won."
  personas: [Dora, Bruno, Ada]
  entry_points:
    - url: "Folders: ~/.compozy/agents|skills, ~/.compozy/profiles/<p>/, <ws>/.compozy/, <ws>/.compozy/profiles/<p>/"
      origin: direct
    - url: "CLI: compozy config path|get|set|unset --scope user|profile|workspace, compozy agent list, compozy skill list"
      origin: direct
    - url: "CLI: compozy secret set|rm, compozy provider inspect"
      origin: direct
    - url: "HTTP and UDS: /api/settings and its sections, vault and provider status routes"
      origin: direct
    - url: "Web Settings: Persona, Hooks, Command palette, Memory, and source badges"
      origin: in-app-nav
    - url: "Native: compozy__config_get|set|unset and memory projections inside a session"
      origin: agent
  actions:
    - step: 1
      verb: "Drop a resource into a profile layer and into a repository per-profile folder"
      expected_observable: "Both appear on the next catalog read with no registration step, and the listing names the winning layer and what it shadowed."
    - step: 2
      verb: "Plant a repository layer for a profile that does not exist yet"
      expected_observable: "The content stays dormant, a diagnostic names the path and the create action, and creating or renaming to that name wakes it without touching the repository files."
    - step: 3
      verb: "Write a config key with and without an explicit scope"
      expected_observable: "The default target is the layer that owns the current context; --scope user|profile|workspace targets that layer; a write shadowed by a more specific layer reports saved but not applied and names the winner."
    - step: 4
      verb: "Try a daemon-identity key on a profile layer"
      expected_observable: "It is refused with profile_config_key_denied and allowed-prefix guidance, and leaves no file or apply-record residue."
    - step: 5
      verb: "Set a provider credential inside a non-default profile"
      expected_observable: "It is stored under the owning profile's vault prefix, the value is never echoed, a process-environment reference is refused, and provider inspect names profile override versus user default while saying native logins stay machine-level."
    - step: 6
      verb: "Remove the override after doing work with it"
      expected_observable: "Removal is acknowledged with its consequence, later work falls back to the user credential, and usage still attributes to the owning profile."
    - step: 7
      verb: "Read and write memory in the profile tier"
      expected_observable: "Entries stay inside their owning profile, an aggregate memory read is refused, and profile-tier reads fail closed while the directory move is still pending."
    - step: 8
      verb: "Rename the profile"
      expected_observable: "Machine folders and vault references rewrite inside the rename transaction, repository folders are only offered as pending edits, extension placements go dormant with hints, and selections are untouched."
  goal:
    observable: "Effective resources, config, MCP entries, credentials, and memory resolve through one documented precedence, and every surface reports the same winning layer."
    side_effects: [config-file-written-in-one-layer, apply-record-naming-the-layer, vault-secret-stored-under-the-owner-prefix, mcp-sidecar-merged-in-its-slot, memory-entry-written-in-the-profile-tier, dormancy-diagnostic-emitted]
  true_end_state: "A fresh read of config, agents, skills, MCP servers, credentials, and memory agrees on every surface; repository files are byte-identical unless the operator accepted a rename and committed it themselves."
  exit:
    natural: "The operator keeps working with a context that has its own material without any of it leaking into another profile or into the repository."
  abandonment:
    - at_step: 3
      how: "The operator interrupts a config write."
      resume: "No partial file and no apply record; the previous good value still resolves."
    - at_step: 2
      how: "The operator ignores the dormant-layer hint indefinitely."
      resume: "Nothing nags and nothing is written; the content wakes the moment a profile with that name exists."
    - at_step: 5
      how: "The operator cancels the credential removal at its confirmation."
      resume: "The override stays in place and work continues to resolve it."
  crosses: [J-operate-profiles, J-adopt-extension-profiles, J-administer-runtime-settings, config-overlay, resource-discovery-roots, vault, providers-prestart-cache, MCP sidecars, memory-store, CLI, HTTP, UDS, Web settings, native-tools]
```
