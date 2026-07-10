---
name: cy-create-techspec
description: Creates a Technical Specification plus its companion test contract (_tests.md) by translating PRD business requirements into implementation designs through interactive technical clarification. Use when a PRD exists and needs a technical plan, when technical architecture decisions need documentation, or when a feature needs its exhaustive test-case catalog. Do not use for PRD creation, task breakdown, or direct code implementation.
argument-hint: "[feature-name] [prd-file]"
---

# Create TechSpec

Translate business requirements into a detailed technical specification and its companion test contract.

<HARD-GATE>
- Explore before designing: every TechSpec MUST be informed by the existing architecture.
- Questions before writing: the user MUST shape the design by answering technical clarification questions — for every TechSpec, however simple. "Simple" changes are where unexamined architecture assumptions cause the most integration failures; brief review is fine, skipped review is not.
- Decide, then write: once the questions are answered and the ADRs are recorded, write the files directly. The user reviews the generated files and requests changes afterward — no draft-approval loops.
</HARD-GATE>

## Full Scope, One Design

Design for the complete PRD scope in this single TechSpec. Agents run long and `cy-create-tasks` decomposes the work later, so design size is never a reason to trim scope or stage the design into phases.

Design minimalism still applies — to the design, never to the scope: include no component, interface, or abstraction the design does not strictly need, and prefer adding a file to an existing package over proposing new packages or directories.

## Asking Questions

Ask every question through the runtime's dedicated interactive question tool — the mechanism that presents a question and pauses execution until the user responds. If the runtime has no such tool, present the question as the complete message and stop generating; never answer a question on the user's behalf.

- One question per message: exactly one question mark, then stop; follow-ups go in the next message, after the user answers.
- Lead with a recommendation: state which option you would pick and why in one line, so the user reacts to a position instead of facing a blank menu.
- Multiple-choice whenever the options can be predetermined, with your recommendation first and a fallback option ("D) Other — describe").
- Never spend a question on what the codebase can answer: explore first; user answers are for genuine trade-offs — priorities, risk appetite, and the product intent behind technical choices.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- Optional: existing `_prd.md` and `_user_stories.md` as primary input.
- Optional: existing `_techspec.md` for update mode.

## Workflow

Track each step as a task in the runtime's task tracker when one is available, and complete the steps in order.

1. Gather context.
   - Read `_prd.md` and `_user_stories.md` from `.compozy/tasks/<name>/` as the primary input. If no PRD exists, ask the user for a description of what needs technical specification.
   - Read existing ADRs from `.compozy/tasks/<name>/adrs/` (create the directory if missing) to understand decisions already made.
   - Spawn an Agent tool call to explore the codebase for architecture patterns, existing components, dependencies, and technology stack.
   - If `_techspec.md` already exists, read it and operate in update mode.

2. Grill the design.
   - Focus on HOW to implement, WHERE components live, and WHICH technologies to use. Map the load-bearing technical decisions into a decision tree — architecture approach and component boundaries, data models and storage, API design and integration points, testing strategy and performance requirements — and walk it branch by branch, resolving dependencies between decisions one at a time.
   - Chase vague answers until each branch is concrete; a load-bearing decision left fuzzy resurfaces as an integration failure.
   - Keep grilling until every branch that shapes the design is resolved or explicitly parked — the question count is an output of the tree, not a budget.

3. Record ADRs for significant technical decisions.
   - For each significant decision (architecture pattern, technology choice, data model approach): read `references/adr-template.md`, determine the next number from the files in `adrs/`, fill the template (chosen design as Decision, rejected alternatives as Alternatives Considered, trade-offs as Consequences; Status "Accepted", Date today), and write `adrs/adr-NNN.md` (zero-padded 3-digit sequential number).
   - Even simple features get at least one ADR documenting the primary technical approach chosen and the alternatives rejected.

4. Write the TechSpec.
   - Read `references/techspec-template.md` and fill every applicable section; the template carries the per-section rules.
   - Map every PRD goal and every story in `_user_stories.md` to a technical component; reference PRD sections by name without duplicating business context.
   - Core Interfaces must show the primary type other components depend on, in the project's primary language.
   - List every ADR in the Architecture Decision Records section; if step 3 produced none, go back and create at least one first.
   - Prefer active voice and definite, specific language; every sentence earns its place. Language: English.
   - Write `.compozy/tasks/<name>/_techspec.md`.

5. Write the test contract.
   - Read `references/tests-template.md` and write `.compozy/tasks/<name>/_tests.md`.
   - Derive unit cases from every component and interface in the TechSpec, including every error path; integration cases from every component boundary and external integration; end-to-end cases from every user journey in `_user_stories.md`.
   - Done when the coverage matrix satisfies the template's Coverage Demands and every case meets its Case-Writing Rules.

6. Hand off.
   - Confirm both file paths to the user and invite change requests directly on the generated files.
   - Point to `cy-create-tasks` as the next step.

## Error Handling

- PRD missing: proceed with user-provided context and note the absence in the Executive Summary.
- `_user_stories.md` missing: derive journeys from the PRD's User Stories section and note the coverage gap in `_tests.md`.
- Conflicting architectural patterns in the codebase: document both and recommend one with rationale.
- Target directory missing: create it.
- Update mode: preserve sections the user has not asked to change, and mirror any behavior or interface change into `_tests.md` so the contract stays in sync.
