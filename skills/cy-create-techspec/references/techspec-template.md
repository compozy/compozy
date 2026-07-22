# TechSpec Template

Use this template to structure every Technical Specification. Fill each section based on technical clarification outcomes and codebase exploration. Omit sections that do not apply and note the reason.

## Executive Summary

Brief technical overview in 1-2 paragraphs:
- Key architectural decisions
- Implementation strategy and approach
- Primary technical trade-offs

## System Architecture

### Component Overview

Main components, their responsibilities, and relationships:
- Component name, purpose, and boundaries
- Data flow between components
- External system interactions

## Implementation Design

### Core Interfaces

Key service interfaces with code examples. Limit each example to 20 lines or fewer:
- Interface definitions and contracts
- Method signatures with parameter and return types
- Error handling conventions

### Data Models

Core domain entities and their relationships:
- Entity definitions with field types
- Request and response types for APIs
- Database schemas or storage structures

### API Endpoints

API surface organized by resource:
- Method, path, and description
- Request format and required fields
- Response format and status codes

## Integration Points

External services and system boundaries. Include only when the design integrates with systems outside the codebase:
- Service name and purpose of integration
- Authentication and authorization approach
- Error handling and retry strategy

## Impact Analysis

Table of components affected by this implementation:

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| [component] | [new/modified/deprecated] | [what changes and risk level] | [action needed] |

### Contract Change Analysis

Required for every change to an existing API, schema, event, interface, or protocol. Derive known consumers from repository analysis. For each changed contract, record:
- Contract diff: name the contract and list the exact fields, routes, methods, or semantics added, removed, or changed
- Active consumers: enumerate every known internal and external consumer with its repository path or ownership evidence; when none are found, state the paths and searches used to establish that result
- Rollout strategy: choose exactly one of atomic consumer updates, backward compatibility, versioning, content negotiation, feature flag, or temporary adapter, then state the rollout order
- Consumer coordination: put consumer updates and verification in the same implementation task or a coordinated task linked by a declared dependency
- Temporary compatibility: for every compatibility layer, record its owner, cleanup condition, and removal task

Block TechSpec generation when an active consumer exists and no rollout strategy is defined.

## Testing Approach

Strategy only — every concrete test case lives in `_tests.md`, the test contract written alongside this TechSpec:
- Frameworks, harnesses, and fixture strategy; fakes sit at I/O boundaries only
- What each level (unit / integration / e2e) covers for this feature and how it runs
- Environment or data dependencies the integration and e2e suites need

## Development Sequencing

### Build Order

Ordered implementation sequence respecting dependencies:
1. [First component] - no dependencies
2. [Second component] - depends on step 1
3. [Continue with dependency chain]

### Technical Dependencies

Blocking dependencies that must be resolved before implementation:
- Infrastructure requirements
- External service availability
- Team deliverables or shared components

## Monitoring and Observability

Operational visibility for the implementation:
- Key metrics to track
- Log events and structured fields
- Alerting thresholds and escalation

### Operational Event Contracts

When a requirement mandates emitting an operational event, include one machine-readable YAML contract per event. Replace every placeholder with a resolved value; use `none` only with an explicit reason. List every emitted payload field, including identifiers, under `fields` so each field has its own requirement, privacy classification, and source.

```yaml
event_contracts:
  - name: "<stable event name>"
    trigger: "<exact emission condition>"
    fields:
      - name: "<payload field name>"
        requirement: "<required|optional>"
        privacy: "<privacy classification>"
        source: "<authoritative value source>"
    forbidden_fields:
      - "<field that must never enter the payload>"
    identifiers:
      request_id:
        field: "<payload field or none>"
        source: "<request ID source or reason none>"
      correlation_id:
        field: "<payload field or none>"
        source: "<correlation ID source or reason none>"
      actor_id:
        field: "<payload field or none>"
        source: "<actor identifier source or reason none>"
      resource_id:
        field: "<payload field or none>"
        source: "<resource identifier source or reason none>"
    allowed_outcomes:
      - "<stable outcome enum>"
    outcome_behavior:
      success: "<when and what to emit, or reason not applicable>"
      rejection: "<when and what to emit, or reason not applicable>"
      replay: "<deduplication and emission behavior, or reason not applicable>"
      stale_command: "<detection and emission behavior, or reason not applicable>"
    delivery_semantics:
      guarantee: "<at-most-once|at-least-once|effectively-once>"
      ordering: "<ordering guarantee>"
      retry: "<retry and deduplication policy>"
    sink_failure:
      behavior: "<drop|retry|fail operation|other explicit policy>"
      caller_impact: "<observable effect on the triggering operation>"
      fallback: "<durable fallback or none with reason>"
```

Any missing or unresolved contract value is a blocking product decision: return to technical clarification and do not publish the TechSpec. Never infer an event value, identifier source, privacy class, outcome, delivery guarantee, or failure policy.

For every event contract, add concrete IDs to `_tests.md` covering:
- Validation of required fields and outcome enums, including missing and unknown values
- Rejection of forbidden payload fields and privacy-policy violations
- Success, rejection, replay, and stale-command behavior
- Delivery guarantees, including duplicate and out-of-order delivery where applicable
- Handling of event-sink failures, including the declared caller impact, retry policy, and fallback

## Technical Considerations

### Key Decisions

Significant technical choices with rationale:
- Decision: what was chosen
- Rationale: why this option
- Trade-offs: what was given up
- Alternatives rejected: what else was considered and why not

### Known Risks

Technical challenges and mitigation strategies:
- Risk description and likelihood
- Mitigation approach
- Areas requiring further research or prototyping

## Architecture Decision Records

ADRs documenting key decisions made during PRD brainstorming and technical design:
- [ADR-NNN: Title](adrs/adr-NNN.md) — One-line summary of the decision
