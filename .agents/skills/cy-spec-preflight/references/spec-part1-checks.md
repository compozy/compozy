# Spec Part I Preflight Checks

Run after `cy-create-spec` writes Part I (Stage 1), before the checkpoint that opens Stage 2.

## MUST contain

- [ ] Problem statement: who feels the pain, why now.
- [ ] User/operator impact: observable from outside the system.
- [ ] Agent/operator manageability outcome: who or what must inspect, configure, operate, or repair the capability outside the web UI.
- [ ] Extension ecosystem expectation: whether runtime/third-party extension points should participate, stated without implementation detail.
- [ ] Goals: bulleted, each one independently observable.
- [ ] **Non-Goals**: bulleted, explicit (not inferred).
- [ ] Success criteria observable from outside the system.
- [ ] Open Questions: captures unresolved product choices without inventing answers.
- [ ] Stage-1 ADRs recorded under `adrs/` for the decided direction and any significant scope decision.
- [ ] Optional: research links, reference implementations under `.resources/`.

## MUST NOT contain (run `scripts/check-spec-part1-leak.py`)

- [ ] Framework names: `react`, `next.js`, `tanstack-query`, `gin`, `cobra`, `gorm`, etc.
- [ ] Storage engine names: `PostgreSQL`, `SQLite`, `Redis`, `S3`, `MySQL`, `BigQuery`.
- [ ] Wire protocols: `gRPC`, `JSON-RPC`, `WebSocket`, `MQTT`.
- [ ] Auth standards: `OAuth 2.0`, `JWT`, `mTLS`, `PKCE`, `SAML`.
- [ ] File formats: `YAML`, `JSON`, `TOML`, `Protobuf`, `XML`.
- [ ] HTTP status codes: explicit numbers (`422`, `503`).
- [ ] SQL schema names, column names, table names.
- [ ] Tool names: `bun`, `mise`, `goreleaser`, `vite`.

## Allowed exceptions (justify in the spec body)

- The spec is *about* Compozy Network's wire format → naming the format is the user-observable surface.
- The spec is about an AGENT.md / MEMORY.md / SKILL.md file format → naming the format is the product.
- The spec scopes a specific framework's ergonomics inside `web/` (e.g., TanStack Query patterns) → naming the framework is the topic.

## Stage 1 gate

The Stage 1 gate is the checkpoint: the user confirms the Stage 1 summary before Stage 2 opens. Cross-LLM peer review targets the complete spec only, after final approval — never a Stage 1 draft.
