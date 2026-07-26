# L-013 — PRD must not name frameworks, storage, error codes, or file formats

**Class:** Spec authoring
**Date discovered:** 2026-04-18 (todo-api smux pairing run)
**Evidence sources:** Codex orchestrator prompt template + analysis_codex_sessions

## Context

In the `todo-api` smux pairing experiment, Pedro built an orchestrator role for one Claude pane that explicitly inspects PRDs and rejects any document that surfaces implementation choices. The orchestrator instruction was: _"PRD naming frameworks/storage engines/file formats — strip, push to TechSpec."_ Pedro adopted this as a generally-applied rule across AGH spec authoring.

A PRD that names `PostgreSQL`, `react-query`, `OAuth 2.0`, `JWT`, `gRPC`, or specific HTTP error codes leaks implementation into the vision document. The implementer reads the PRD with framework constraints already locked in and stops asking "is this the right shape?" The TechSpec phase exists exactly to make those decisions — moving them earlier collapses two phases into one and removes the option to choose differently when the architecture surface comes into focus.

## Root cause

LLM-authored PRDs default to "concrete and useful" framing because that's how product writing reads online. Real product writing is meant to ship a feature; AGH PRDs feed into a TechSpec that an architecture-aware reviewer will pressure-test. The PRD's job is to constrain the _user-observable_ surface, not the _implementation_ surface.

## Rule

> PRDs frame **what** and **why**, never **how**. PRDs MUST NOT name:
>
> - Frameworks (`react`, `next.js`, `tanstack-query`, `gin`, `cobra`, `gorm`).
> - Storage engines (`PostgreSQL`, `SQLite`, `Redis`, `S3`).
> - Wire protocols (`gRPC`, `JSON-RPC`, `WebSocket`).
> - Auth standards (`OAuth 2.0`, `JWT`, `mTLS`, `PKCE`).
> - File formats (`YAML`, `JSON`, `TOML`, `Protobuf`).
> - HTTP error codes / status numbers.
> - SQL schema or column names.
> - Specific tools (`bun`, `mise`, `goreleaser`).
>
> Strip and push to the TechSpec. Approval owner: PRD author.

## Operationalization

`cy-spec-preflight` runs a regex pass over the PRD draft and surfaces any matching tokens. Items found are listed for the author to either justify (rare exception, e.g., when the PRD is _about_ AGH Network's wire format) or strip.

PRDs may name **product surfaces** (CLI verb, web route, doc page) when those are user-observable. They may not name the implementation behind those surfaces.

## Anti-patterns

- "Use PostgreSQL for the durable queue." → strip; TechSpec decides.
- "Return 422 when validation fails." → strip; TechSpec decides.
- "JWT-based session tokens." → strip; TechSpec decides.
- "React-query mutation for the publish action." → strip; TechSpec decides.
- "Store config in a YAML file." → strip; TechSpec decides whether YAML, TOML, or sidecar JSON.

## Allowed exceptions

- AGH Network protocol PRDs that are _about_ wire format (capability envelopes, carrier-neutral routing fields). The protocol IS the user-observable surface for that PRD.
- AGENT.md / MEMORY.md / SKILL.md PRDs where the file format is the product.
- PRDs scoped to a specific framework's ergonomics (e.g., a PRD about TanStack Query usage patterns inside `web/`).

## Source

- `~/.codex/sessions/2026/04/18/19-27-52` (smux pairing orchestrator instruction)
- `../analysis/analysis_codex_sessions.md` §Anti-Patterns (todo-api smux rule)
- `docs/_memory/_synthesis.md` Top-level Finding 3
