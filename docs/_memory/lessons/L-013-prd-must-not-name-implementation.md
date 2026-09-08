# L-013 — PRD must not name frameworks, storage, error codes, or file formats

**Class:** Spec authoring
**Date discovered:** 2026-04-18 (todo-api smux pairing run)
**Evidence sources:** Codex orchestrator prompt template + analysis_codex_sessions

## Context

In the `todo-api` smux pairing experiment, Pedro built an orchestrator role for one Claude pane that explicitly inspects PRDs and rejects any document that surfaces implementation choices. The orchestrator instruction was: _"PRD naming frameworks/storage engines/file formats — strip, push to TechSpec."_ Pedro adopted this as a generally-applied rule across Compozy spec authoring.

A PRD that names `PostgreSQL`, `react-query`, `OAuth 2.0`, `JWT`, `gRPC`, or specific HTTP error codes leaks implementation into the vision document. The implementer reads the PRD with framework constraints already locked in and stops asking "is this the right shape?" The TechSpec phase exists exactly to make those decisions — moving them earlier collapses two phases into one and removes the option to choose differently when the architecture surface comes into focus.

## Root cause

LLM-authored PRDs default to "concrete and useful" framing because that's how product writing reads online. Real product writing is meant to ship a feature; Compozy PRDs feed into a TechSpec that an architecture-aware reviewer will pressure-test. The PRD's job is to constrain the _user-observable_ surface, not the _implementation_ surface.

## Rule

> Product requirements explain observable outcomes and why they matter. Technical implementation decisions belong in the technical section unless they are themselves a public contract, a fixed user constraint, or the subject of the product requirement.

## Operationalization

Treat implementation-keyword scans as advisory prompts to inspect context. Move incidental library/schema choices to the technical design; retain user-facing CLI paths, protocols, formats, and constraints when they define acceptance. Vocabulary alone is not a blocking failure.

## Anti-patterns

- "Use PostgreSQL for the durable queue." → strip; TechSpec decides.
- "Return 422 when validation fails." → strip; TechSpec decides.
- "JWT-based session tokens." → strip; TechSpec decides.
- "React-query mutation for the publish action." → strip; TechSpec decides.
- "Store config in a YAML file." → strip; TechSpec decides whether YAML, TOML, or sidecar JSON.

## Allowed exceptions

- Compozy Network protocol PRDs that are _about_ wire format (capability envelopes, carrier-neutral routing fields). The protocol IS the user-observable surface for that PRD.
- AGENT.md / MEMORY.md / SKILL.md PRDs where the file format is the product.
- PRDs scoped to a specific framework's ergonomics (e.g., a PRD about TanStack Query usage patterns inside `web/`).

## Source

- `~/.codex/sessions/2026/04/18/19-27-52` (smux pairing orchestrator instruction)
- `../analysis/analysis_codex_sessions.md` §Anti-Patterns (todo-api smux rule)
- `docs/_memory/_synthesis.md` Top-level Finding 3
