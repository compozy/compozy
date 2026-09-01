# Spec Template

Structure for `_spec.md` — the single specification consumed by LLM agents downstream (`cy-create-tasks`, `cy-execute-task`, review rounds, `cy-spec-peer-review`). Part I supplies business rules, domain behavior, and product intent; Part II supplies the implementation design. Fill each section per its rules; omit a Part II section only when it does not apply, and note the reason. Leave insufficiently-known material in Open Questions rather than guessing.

---

# Part I — Product

Part I frames WHAT and WHY. Implementation choices — frameworks, storage engines, file formats, transports, error codes, schema details — belong to Part II; naming them here is a defect unless the feature is _about_ that technology.

## Overview

- **Motivating Problem**: the concrete pain that triggered this spec and the simplest end-to-end behavior that solves it, stated as an outcome observable from outside the system. Slice 1 of the task breakdown delivers it; an ADR that narrows or defers it is valid only with the user's sign-off recorded in the ADR itself.
- Who it is for
- Why it is valuable

## Goals

Product outcomes stated as observable behavior, not metrics:

- What users can do after this ships that they could not do before
- What the system guarantees or enforces once the feature exists
- What becomes unnecessary, automatic, or impossible for users

## User Stories

Index into `_user_stories.md`, the canonical story catalog — do not restate stories here:

- One line per feature area: the `US-NNN` range it covers and its theme
- Link the catalog: [Full user stories](_user_stories.md)

## Core Features

- Feature name: what it does, why it is important, high-level behavior
- Functional requirements for each feature
- Interaction between features

## Business Rules

Domain rules the implementation must enforce, stated precisely:

- Invariants that must always hold (e.g., "a run belongs to exactly one workspace")
- Validation rules and their user-facing outcomes
- Permission and visibility rules per persona
- Lifecycle and state-transition rules (which states exist, what may move where, and when)
- Calculations, limits, and defaults with their exact values

## User Experience

User journey from first contact to regular use:

- Key personas and their goals
- Primary user flows step by step
- Accessibility requirements
- Onboarding and discoverability

For UI-bearing features the screen-by-screen change map lives in `_uiux.md`; keep journeys and personas here and index the surface map instead of restating it.

## High-Level Technical Constraints

Required boundaries that shape the product without prescribing implementation:

- Required integrations with existing systems
- Compliance mandates or regulatory requirements
- Performance targets from a user perspective
- Data privacy and security requirements
- Agent/operator manageability outcome: who or what must inspect, configure, operate, or repair the capability outside the web UI
- Extension ecosystem expectation: whether third-party/runtime extension points should participate

## Non-Goals (Out of Scope)

Capabilities the user decided this feature will not include:

- Adjacent problems that will not be addressed, and why
- Boundaries of this effort

Exclusions record user decisions, never size management: a wanted capability stays in scope — when the slice budget overflows, it moves to a named follow-up spec in the program, never silently out of scope.

## Open Questions

Remaining items that need clarification — unclear requirements, edge cases requiring stakeholder input, dependencies on decisions not yet made. Never park a question a recorded ADR or Non-Goal already answers.

---

# Part II — Technical

Part II designs to serve the frozen surface: every section must be consistent with `_dx.md` (and `_uiux.md` when present). Six quality markers are mandatory (`cy-spec-preflight` validates them): **MVP Boundary** · **Architectural Boundaries** · **concrete Go interface signatures** · **data-model field rationale** · **side-table-vs-JSON decisions** · **numbered safety invariants**.

## Executive Summary

Brief technical overview in 1-2 paragraphs: key architectural decisions, implementation strategy, primary trade-offs. When the spec attacks a real incident, open with the confirmed reproduction (timestamp, command, observed evidence).

## MVP Boundary

One statement naming which numbered tasks compose the MVP, which slice ships first and how it solves the Motivating Problem, what is post-MVP, and what is explicitly out of scope.

## Developer Experience

Index into the frozen surface contracts — do not restate them:

- [Developer experience contract](_dx.md) — one line per surface it covers (YAML, CLI, API, SDK, config, native tools)
- [UI change map](_uiux.md) — only when the feature is UI-bearing

## System Architecture

Main components, their responsibilities, and relationships:

- Component name, purpose, and boundaries
- Data flow between components
- External system interactions

## Architectural Boundaries

Which packages can and cannot import which. Name new internal packages explicitly. Reference the `daemon/` composition root.

## Implementation Design

### Core Interfaces

Critical interfaces pasted as code blocks in the project's primary language — every method signature final, each example 20 lines or fewer. Show the primary type other components depend on.

### Data Models

Core domain entities and their relationships:

- Every new SQLite column, frontmatter field, or config key listed with purpose and shape (`field TYPE — purpose` or a column table)
- Side-table-vs-JSON decision stated for every new domain entity: side-tables for matchable state, JSON for opaque metadata only, with the reason
- Request and response types for APIs

### API Endpoints

Internal design of the API surface `_dx.md` promises — handlers, validation, status codes, error shapes. The client-visible request/response examples live in `_dx.md`; keep this section consistent with them.

## Integration Points

External services and system boundaries. Include only when the design integrates with systems outside the codebase: service and purpose, authentication approach, error handling and retry strategy.

## Impact Analysis

| Component   | Impact Type               | Description and Risk          | Required Action |
| ----------- | ------------------------- | ----------------------------- | --------------- |
| [component] | [new/modified/deprecated] | [what changes and risk level] | [action needed] |

Breaking changes list their **delete targets** — code, storage, APIs, CLI, extensions, specs, and `.compozy/tasks/*` artifacts that disappear in the same change. Include "no fallback / no compat shim / no placeholder" clauses where drift is likely.

## Extensibility Integration Plan

Extension manifests, hooks, skills/capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, and protocol docs that are added/changed/removed — or explicitly unaffected, naming the surfaces checked and why.

## Agent Manageability Plan

CLI verbs, HTTP endpoints, UDS routes, structured outputs, status/config discovery, and deterministic errors agents will use to operate the feature. UI-only control is incomplete. Keep consistent with `_dx.md`.

## Config Lifecycle

`config.toml` keys/defaults, merge/overlay behavior, validation, examples, generated CLI/site docs, and tests that are added/changed/removed — or explicitly unaffected, naming the surfaces checked.

## Testing Approach

Strategy only — every concrete test case lives in `_tests.md`:

- Frameworks, harnesses, and fixture strategy; fakes sit at I/O boundaries only
- What each level (unit / integration / e2e) covers for this feature and how it runs
- Environment or data dependencies the integration and e2e suites need

## Development Sequencing

### Build Order

The dependency map that feeds `_tasks.md` edges: which contracts must exist before which consumers, each phase with its own verification gate; safe cleanup phases separated from behavior-changing edits. Delivery sequence belongs to the task breakdown, which orders shippable slices by user value — slice 1 solves the Motivating Problem.

### Technical Dependencies

Blocking dependencies that must be resolved before implementation: infrastructure requirements, external service availability, shared components.

## Monitoring and Observability

Key metrics, log events and structured fields, alerting thresholds and escalation.

## Technical Considerations

### Key Decisions

Decision, rationale, trade-offs, alternatives rejected — for every significant choice not already carrying its own ADR.

### Known Risks

Risk and likelihood, mitigation approach, areas requiring further research or prototyping.

## Safety Invariants

For concurrency- or ownership-sensitive paths: the invariants as a numbered list, never prose. Omit only when the design has no such path, and say so.

## File References

The read-first index for implementing agents: every path the design depends on, each with one clause on why to read it. Use `path:line-start-line-end` when a region carries the pattern. Index an existing per-file catalog (e.g. `analysis/` reference tables) instead of inlining it, and name sources that are not navigable ("cited by concept — not vendored under `.resources/`") so nobody hunts for a missing path. `cy-create-tasks` copies each task's subset from here into its `### Relevant Files` / `### Dependent Files` / `### Competitor References`.

### Repo Files

- `internal/<pkg>/<file>.go:120-180` — what it establishes and why the design depends on it

### Competitor References

Include only when the design draws on vendored competitor sources; paths stay relative to `.resources/`, never paraphrased:

- `.resources/<repo>/<path>:100-150` — the pattern to mirror, or the mismatch to reject, and why

### Design and Analysis Sources

- `analysis/<NN_analysis_topic>.md` — the decision context it carries and the ADRs it feeds
- `docs/design/opendesign/<artboard>.html` — the surface it is the visual contract for

## Assumptions and Defaults

Closing section pre-empting "what if" questions: every assumption made and every default chosen, with the value.

## Architecture Decision Records

Every ADR from both stages:

- [ADR-NNN: Title](adrs/adr-NNN.md) — One-line summary of the decision
