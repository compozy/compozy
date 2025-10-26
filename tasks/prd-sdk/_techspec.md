# Technical Specification: Compozy GO SDK (Referenced)

## Executive Summary

This Tech Spec formalizes the implementation approach for the Compozy GO SDK. It adheres to the PRD modules under `tasks/prd-sdk/` and references them directly instead of duplicating content. Architectural decisions, APIs, and plans are defined in the PRD documents and are cited by path throughout this spec.

- Executive goals and motivation: see 01-executive-summary.md: tasks/prd-sdk/01-executive-summary.md
- Final architecture and integration approach: tasks/prd-sdk/02-architecture.md
- API surface (builders, types, patterns): tasks/prd-sdk/03-sdk-entities.md
- Delivery plan and sequencing: tasks/prd-sdk/04-implementation-plan.md
- Representative usage and edge-case examples: tasks/prd-sdk/05-examples.md
- Migration strategy (YAML → SDK): tasks/prd-sdk/06-migration-guide.md
- Testing strategy and coverage targets: tasks/prd-sdk/07-testing-strategy.md
- Review + closure tracking: tasks/prd-sdk/PLAN_REVIEW.md, tasks/prd-sdk/08-completion-summary.md, tasks/prd-sdk/00-COMPLETION-REPORT.md

## System Architecture

### Domain Placement

Follow the domain mapping and module boundaries defined here: tasks/prd-sdk/02-architecture.md

Key engine domains and their ownership (summarized; details in Architecture doc):
- agent, task, workflow, tool, runtime, knowledge, memory, mcp, schema, project, infra, core (see Architecture “Go Workspace Structure” and “Module Dependencies & Integration”): tasks/prd-sdk/02-architecture.md
- SDK package layout and one-way import rules (sdk → engine only): tasks/prd-sdk/02-architecture.md

### Component Overview

Component responsibilities, relationships, and data/control flow are specified in the Integration Layer and Context-First sections:
- SDK → Engine Integration and resource registration: tasks/prd-sdk/02-architecture.md
- Context-first pattern enforcement (logger/config/validation): tasks/prd-sdk/02-architecture.md
- Task types (9), unified Signal, Native Tools: tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md

## Implementation Design

### Core Interfaces

The builder APIs and their Build(ctx) contracts are defined per category in the API reference:
- Project/Workflow/Agent/Task/… builders: tasks/prd-sdk/03-sdk-entities.md
- Unified SignalBuilder (send/wait), NativeToolsBuilder, MCP and Memory builders: tasks/prd-sdk/03-sdk-entities.md

Example signatures and usage are available in the Examples collection: tasks/prd-sdk/05-examples.md

## Planning Artifacts (Referenced)

Per the process, planning artifacts exist in the PRD set and should be treated as authoritative:
- Implementation plan (phases, critical path, parallel lanes): tasks/prd-sdk/04-implementation-plan.md
- Example coverage and runnable scenarios: tasks/prd-sdk/05-examples.md
- Testing strategy and coverage matrix: tasks/prd-sdk/07-testing-strategy.md
 - Docs plan for NEW top-level SDK section (tabs + pages structure): tasks/prd-sdk/_docs.md

If separate companion files (`_docs.md`, `_examples.md`, `_tests.md`) are desired, they can be generated later from templates in tasks/docs/*. For now, this Tech Spec references the authoritative PRD modules to avoid duplication.

### Data Models

All configuration structs, identifiers, and modes are the engine’s types consumed by the SDK; see:
- Engine types consumed by SDK (imports): tasks/prd-sdk/02-architecture.md
- Builder-produced config structures across categories: tasks/prd-sdk/03-sdk-entities.md

### API Endpoints

Endpoints and client usage are demonstrated here:
- Client usage patterns (deploy/execute/status): tasks/prd-sdk/01-executive-summary.md and tasks/prd-sdk/06-migration-guide.md
- Server/embedded lifecycle and router integration: tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/05-examples.md

## Integration Points

External integrations are covered by the PRD and should be implemented per their guidance:
- MCP transports (stdio/SSE/HTTP), headers, proto, sessions: tasks/prd-sdk/03-sdk-entities.md and tasks/prd-sdk/02-architecture.md
- Knowledge embedders/vector DBs, ingestion/retrieval: tasks/prd-sdk/03-sdk-entities.md and tasks/prd-sdk/05-examples.md
- Memory backends and token counting: tasks/prd-sdk/03-sdk-entities.md

## Impact Analysis

Impacted areas, risks, and mitigations are captured across the review, architecture, and implementation plan. Use these as the source of truth during implementation planning:

| Affected Component | Type of Impact | Reference |
| --- | --- | --- |
| engine/task (all 9 types) | API conformance and validation alignment | tasks/prd-sdk/02-architecture.md; tasks/prd-sdk/03-sdk-entities.md |
| engine/infra/server, resource store | Registration, validation, hybrid SDK+YAML | tasks/prd-sdk/02-architecture.md |
| knowledge/memory/mcp | Full-surface configuration exposure | tasks/prd-sdk/03-sdk-entities.md |
| docs + examples | Updates and runnable scenarios | tasks/prd-sdk/05-examples.md |
| migration & compatibility | Hybrid projects, phased rollout | tasks/prd-sdk/06-migration-guide.md |

A complete risk/priority breakdown: tasks/prd-sdk/PLAN_REVIEW.md and tasks/prd-sdk/08-completion-summary.md

## Testing Approach

### Unit Tests

- Builder table-driven tests, error aggregation, validation boundaries: tasks/prd-sdk/07-testing-strategy.md
- All Build(ctx) calls and t.Context()/b.Context() usage: tasks/prd-sdk/07-testing-strategy.md

### Integration Tests

- SDK → Engine registration, execution, external services (DB/Redis/MCP) when applicable: tasks/prd-sdk/07-testing-strategy.md and tasks/prd-sdk/05-examples.md

## Development Sequencing

### Build Order

- Phase breakdown, critical path, and parallel lanes: tasks/prd-sdk/04-implementation-plan.md
- Success criteria and gates per phase: tasks/prd-sdk/04-implementation-plan.md

### Technical Dependencies

- Workspace (go.work), module import strategy, context-first rules: tasks/prd-sdk/02-architecture.md
- Version compatibility matrix: tasks/prd-sdk/04-implementation-plan.md

## Monitoring & Observability

- Metrics/tracing/logging guidance and SDK observability points: tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
- Monitoring builder and examples: tasks/prd-sdk/03-sdk-entities.md and tasks/prd-sdk/05-examples.md

## Technical Considerations

### Key Decisions

- Go workspace with SDK module; direct engine types (no YAML intermediate): tasks/prd-sdk/02-architecture.md
- Context-first builder contracts with Build(ctx) everywhere: tasks/prd-sdk/02-architecture.md
- Nine task types aligned with engine; unified Signal; Native tools exposed: tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md

### Known Risks

- Integration complexity of registration flows and hybrid SDK+YAML projects
- API drift between SDK and engine types
- External integration variability (MCP transports, vector DBs)

Risks and mitigations are listed in: tasks/prd-sdk/PLAN_REVIEW.md and tasks/prd-sdk/08-completion-summary.md

### Special Requirements

- Performance expectations and benchmarks, plus success metrics: tasks/prd-sdk/04-implementation-plan.md
- Security/Privacy notes for memory/knowledge/MCP: tasks/prd-sdk/03-sdk-entities.md and tasks/prd-sdk/07-testing-strategy.md

### Standards Compliance

Conformance to repository standards is enforced across docs and examples; see:
- Architecture and coding standards: .cursor/rules/architecture.mdc, .cursor/rules/go-coding-standards.mdc
- Logger and config rules (context-first): .cursor/rules/logger-config.mdc, .cursor/rules/global-config.mdc
- Testing rules and patterns: .cursor/rules/test-standards.mdc
- PRD module alignment statements: tasks/prd-sdk/08-completion-summary.md, tasks/prd-sdk/00-COMPLETION-REPORT.md
