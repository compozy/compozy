# PRD Preflight Checks

Run after `cy-create-prd` produces a draft, before approval.

## MUST contain

- [ ] Problem statement: who feels the pain, why now.
- [ ] User/operator impact: observable from outside the system.
- [ ] Agent/operator manageability outcome: who or what must inspect, configure, operate, or repair the capability outside the web UI.
- [ ] Extension ecosystem expectation: whether runtime/third-party extension points should participate, stated without implementation detail.
- [ ] Goals: bulleted, each one independently observable.
- [ ] **Non-Goals**: bulleted, explicit (not inferred).
- [ ] Success Criteria: measurable.
- [ ] Open Questions: captures unresolved product choices without inventing answers.
- [ ] Architecture Decision Records: links any PRD decision ADRs created by `cy-create-prd`.
- [ ] Optional: research links, reference implementations under `.resources/`.

## MUST NOT contain (run `scripts/check-prd-implementation-leak.py`)

- [ ] Framework names: `react`, `next.js`, `tanstack-query`, `gin`, `cobra`, `gorm`, etc.
- [ ] Storage engine names: `PostgreSQL`, `SQLite`, `Redis`, `S3`, `MySQL`, `BigQuery`.
- [ ] Wire protocols: `gRPC`, `JSON-RPC`, `WebSocket`, `MQTT`.
- [ ] Auth standards: `OAuth 2.0`, `JWT`, `mTLS`, `PKCE`, `SAML`.
- [ ] File formats: `YAML`, `JSON`, `TOML`, `Protobuf`, `XML`.
- [ ] HTTP status codes: explicit numbers (`422`, `503`).
- [ ] SQL schema names, column names, table names.
- [ ] Tool names: `bun`, `mise`, `goreleaser`, `vite`.

## Allowed exceptions (justify in PRD body)

- The PRD is *about* AGH Network's wire format → naming the format is the user-observable surface.
- The PRD is about an AGENT.md / MEMORY.md / SKILL.md file format → naming the format is the product.
- The PRD scopes a specific framework's ergonomics inside `web/` (e.g., TanStack Query patterns) → naming the framework is the topic.

## Pre-approval gate

Do not run `cy-spec-peer-review` for PRDs. The PRD approval gate is user approval after the complete draft is reviewed. Cross-LLM peer review is reserved for TechSpecs.
