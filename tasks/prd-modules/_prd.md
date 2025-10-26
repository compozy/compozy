# Product Requirements Document (PRD): Compozy v2 Go SDK

## Overview

A first‑class, type‑safe Go SDK to build and run Compozy projects programmatically (no YAML), with context‑first APIs and direct integration to the engine. This PRD aggregates and references the detailed modules under `tasks/prd-modules/` to remain DRY.

- Problem/why now, audience, value: see tasks/prd-modules/01-executive-summary.md
- Architecture overview and boundaries: tasks/prd-modules/02-architecture.md

## Goals

- Deliver a complete v2 SDK that covers 16 categories / 30 builders (parity with YAML). Source: tasks/prd-modules/03-sdk-entities.md
- Ensure context‑first ergonomics (Build(ctx)), zero global singletons. Source: tasks/prd-modules/02-architecture.md
- Provide runnable examples and migration paths. Sources: tasks/prd-modules/05-examples.md, 06-migration-guide.md
- Ship a new top‑level “SDK” docs section (planning only in this repo). Source: tasks/prd-modules/_docs.md
- Success metrics (post‑release): adoption, example execution rate, doc engagement. See “Success Metrics” below.

## User Stories

Primary personas: Go backend engineers, platform teams embedding Compozy.

- As a Go engineer, I can construct projects/workflows/agents/tasks via type‑safe builders to avoid YAML errors. Examples: tasks/prd-modules/05-examples.md
- As a platform team, I can embed Compozy in my service and execute workflows programmatically. Architecture: tasks/prd-modules/02-architecture.md
- As a migrating user, I can convert YAML projects to SDK gradually (hybrid). Guide: tasks/prd-modules/06-migration-guide.md

## Core Features

Feature set references the SDK inventory and examples; functional requirements are enumerated in these docs:

1) SDK Entity Coverage (16 categories/30 builders) — tasks/prd-modules/03-sdk-entities.md
2) Context‑first, error aggregation (BuildError) — tasks/prd-modules/02-architecture.md
3) 9 Task Types (Signal unified) — tasks/prd-modules/02-architecture.md, 03-sdk-entities.md
4) Memory/MCP/Runtime (native tools) full surface — tasks/prd-modules/03-sdk-entities.md
5) Embedded engine lifecycle + client usage — tasks/prd-modules/02-architecture.md, 01-executive-summary.md
6) Examples gallery (10+ runnable) — tasks/prd-modules/05-examples.md

## User Experience

- Developer UX: fluent builders, context‑first; examples show end‑to‑end usage. Source: tasks/prd-modules/05-examples.md
- Migration UX: side‑by‑side patterns and hybrid flows. Source: tasks/prd-modules/06-migration-guide.md
- Docs UX: NEW “SDK” top‑level section (plan). Source: tasks/prd-modules/_docs.md

## High‑Level Technical Constraints

- Go 1.25.2; context‑first patterns and logging/config via context. Source: tasks/prd-modules/02-architecture.md
- One‑way dep: v2 imports engine types; engine never imports v2. Source: 02-architecture.md
- Direct integration (no YAML intermediate) with resource registration in the engine. Source: 02-architecture.md
- Security/privacy: memory, knowledge, MCP configured per engine semantics. Source: 03-sdk-entities.md

## Non‑Goals (Out of Scope)

- Changing engine semantics or YAML loader behavior (SDK is additive). Source: 01-executive-summary.md
- Replacing REST/CLI docs; SDK docs are a new section, not a rewrite. Source: _docs.md
- Introducing non‑Go SDKs in this cycle (future work).

## Phased Rollout Plan

- MVP and phases map 1:1 to implementation plan. Source: tasks/prd-modules/04-implementation-plan.md
- Phase 0 prototype gate (integration validation) is mandatory. Source: 04-implementation-plan.md

## Success Metrics

- Adoption: # of projects using SDK (goal: 10+ within first month of beta)
- Example execution success: run rate and error rate across provided examples (goal: >95% success internally)
- Docs engagement: page views/time‑on‑page for SDK overview/getting‑started
- Parity: 100% of YAML features represented (tracked against 16/30 inventory)

## Risks and Mitigations

- API drift between engine and SDK → Mitigation: single source of truth for types; integration tests. Sources: 02-architecture.md, 07-testing-strategy.md
- Complex external integrations (MCP/vector DBs) → Mitigation: examples + env‑gated tests. Sources: 05-examples.md, 07-testing-strategy.md
- Docs fragmentation → Mitigation: DRY docs plan referencing PRD; single SDK top‑level section. Source: _docs.md
- Detailed risk matrix: tasks/prd-modules/PLAN_REVIEW.md, 08-completion-summary.md

## Open Questions

- Any additional providers or transports needed for launch? (Ref: 03-sdk-entities.md)
- Any specific IDE integrations needed beyond standard GoDoc and examples?
- SDK versioning cadence vs engine releases? (Ref: 04-implementation-plan.md)

## Appendix

- Background analysis, gaps, and resolutions: tasks/prd-modules/PLAN_REVIEW.md, 08-completion-summary.md, 00-COMPLETION-REPORT.md
- Full architecture reference: 02-architecture.md
- Full API inventory: 03-sdk-entities.md

## Planning Artifacts (Post‑Approval)

- Tech Spec: tasks/prd-modules/_techspec.md
- Docs Plan: tasks/prd-modules/_docs.md (SDK becomes new top‑level tab in docs site; implementation done in a follow‑up PR to docs/)
- Examples Plan: tasks/prd-modules/_examples.md
- Tests Plan: tasks/prd-modules/_tests.md
