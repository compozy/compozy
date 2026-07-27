# J-validate-compozy-hard-cut — Prove the Compozy identity has no legacy fallback

A runtime administrator starts from isolated state and proves that the executable, storage,
environment, native-tool, wire, claim-token, official-skill, and public-brand contracts expose one
Compozy identity. Legacy identifiers fail explicitly or are ignored; they never alias, redirect
runtime state, or leak into persisted or generated output.

```mermaid
flowchart TD
    A[Entry: fresh isolated COMPOZY_HOME and workspace] --> B[Build and start compozy]
    B --> C{Inspect identity surface}
    C -->|runtime state| D[.compozy home + compozy.db + events.db + COMPOZY_* + Compozy helpers]
    C -->|agent plane| E[compozy__ tools + compozy_host__ MCP + min_compozy_version + official compozy skill]
    C -->|wire/security| F[compozy-network/v0 + compozy.* keys + compozy_claim_ redaction]
    C -->|public surface| G[@compozy packages + canonical OpenAPI + local site, metadata, launch route, and docs]
    D --> H[Compare CLI, HTTP, UDS, disk, logs, and doctor]
    E --> H
    F --> H
    G --> H
    H --> I{Try legacy identifier}
    I -->|retired command/home/db/tool/metadata/wire| J[Unknown, ignored, or absent — never aliased]
    I -->|raw claim in mixed case| K[Redacted before log, event, stream, or persistence]
    J --> L[Restart and reopen]
    K --> L
    L --> M[True end: all fresh reads expose only Compozy and both workspaces remain isolated]
    B -.->|legacy state blocks confidence| X[Abandon: preserve evidence; do not add a compatibility reader]
    X -.->|new isolated home| A
```

```yaml
journey:
  id: J-validate-compozy-hard-cut
  name: "Prove the Compozy identity has no legacy fallback"
  value_statement: "An operator and an autonomous agent can trust that every runtime and public identity is Compozy, with no hidden legacy alias or state merge."
  personas: [Dora, Ada, Cora]
  entry_points:
    - url: "COMPOZY_HOME=<isolated> compozy daemon start; compozy status|doctor -o json"
      origin: direct
    - url: "CLI, HTTP, UDS, native-tool, hosted-MCP, and on-disk identity surfaces"
      origin: direct
    - url: "local packages/site build and canonical https://compozy.com metadata declarations"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Start a fresh isolated runtime and inspect its files and structured status"
      expected_observable: "Only .compozy, compozy.db, events.db, Compozy sockets/logs/helpers, and the selected COMPOZY_* environment contracts appear"
    - step: 2
      verb: "Discover and invoke the agent-facing contracts"
      expected_observable: "Only compozy__, compozy_host__, min_compozy_version, metadata.compozy, @compozy packages, canonical OpenAPI artifacts, and the single official compozy skill are offered consistently"
    - step: 3
      verb: "Inspect wire identity and plant a mixed-case claim token"
      expected_observable: "The current wire/version/key family persists; the raw compozy_claim_ token is absent from every durable and streamed surface"
    - step: 4
      verb: "Render the local public surfaces and exercise legacy negative controls"
      expected_observable: "The canonical Compozy origin and launch route agree; legacy commands, home variables, databases, tools, metadata, and wire identifiers never alias"
  goal:
    observable: "Fresh CLI, HTTP, UDS, native, Web, docs, disk, event, and protocol reads expose one Compozy identity"
    side_effects: [isolated-compozy-runtime-state, redacted-claim-records]
  true_end_state: "After restart, all current reads remain Compozy-only, no legacy state was created or opened, the claim secret has zero raw hits, and a second workspace cannot read the first workspace's data."
  exit:
    natural: "The administrator archives the cross-surface identity and redaction evidence for the beta candidate."
  abandonment:
    - at_step: 1
      how: "Unexpected legacy state or a collision makes the fresh runtime ambiguous."
      resume: "Keep the evidence, tear down the lab, and retry only from a new isolated home; never enable a compatibility fallback."
    - at_step: 3
      how: "A raw claim token appears in any output."
      resume: "Stop the session immediately and register the security defect before any further journey work."
  crosses: [CLI, HTTP, UDS, native-tools, hosted-MCP, runtime-home, SQLite, network-wire, redaction, bundled-skill, web, docs, generated-contracts]
```

## Coverage contract

- Safety invariants: 7, 10, 13; hard-cut delete targets from the TechSpec.
- ADRs: ADR-005 and ADR-006, plus the wire and skill hard-cut decisions in Tasks 02–04.
- Deferred: live config translation and first-boot legacy-state migration remain Task 14 work.
