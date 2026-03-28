---
name: create-techspec
description: Creates a Technical Specification by translating PRD business requirements into implementation designs through interactive technical clarification. Use when a PRD exists and needs a technical plan, or when technical architecture decisions need documentation. Do not use for PRD creation, task breakdown, or direct code implementation.
---

# Create TechSpec

Translate business requirements into a detailed technical specification.

## Required Inputs

- Feature name identifying the `tasks/prd-<name>/` directory.
- Optional: existing `_prd.md` as primary input.
- Optional: existing `_techspec.md` for update mode.

## Workflow

1. Gather context.
   - Check for `_prd.md` in `tasks/prd-<name>/`. If it exists, read it as the primary input.
   - If no PRD exists, ask the user for a description of what needs technical specification.
   - Read existing ADRs from `tasks/prd-<name>/adrs/` to understand decisions already made during PRD creation.
   - Create `tasks/prd-<name>/adrs/` directory if it does not exist.
   - Spawn an Agent tool call to explore the codebase for architecture patterns, existing components, dependencies, and technology stack.
   - If `_techspec.md` already exists, read it and operate in update mode.

2. Ask technical clarification questions.
   - Focus on HOW to implement, WHERE components live, and WHICH technologies to use.
   - Cover architecture approach and component boundaries.
   - Cover data models and storage choices.
   - Cover API design and integration points.
   - Cover testing strategy and performance requirements.
   - Ask at most 3 questions per round and wait for answers before the next round.

3. Present the design proposal for approval.
   - System architecture overview with component relationships.
   - Key interfaces and data models.
   - Implementation sequencing with dependencies.
   - Trade-offs considered and risks identified.
   - Wait for user approval before writing the document.
   - If the user requests changes, revise and present again.
   - After the user approves the design, create an ADR for each significant technical decision (architecture pattern chosen, technology selected, data model approach, etc.):
     - Read `references/adr-template.md`.
     - Determine the next ADR number by listing existing files in `tasks/prd-<name>/adrs/`.
     - Fill the template: the chosen design as "Decision", rejected alternatives as "Alternatives Considered", and trade-offs as "Consequences". Set Status to "Accepted" and Date to today.
     - Write each ADR to `tasks/prd-<name>/adrs/adr-NNN.md` (zero-padded 3-digit sequential number).

4. Generate the TechSpec document.
   - Read `references/techspec-template.md` and fill every applicable section.
   - Include an "Architecture Decision Records" section listing all ADRs (from both PRD brainstorming and technical design) with their numbers, titles, and one-line summaries as links to the `adrs/` directory.
   - Write the completed document to `tasks/prd-<name>/_techspec.md`.
   - Every PRD goal and user story should map to a technical component.
   - Reference PRD sections by name but do not duplicate business context.
   - Include code examples only for core interfaces, limited to 20 lines each.

## Error Handling

- If the PRD is missing, proceed with user-provided context and note the absence in the Executive Summary.
- If codebase exploration reveals conflicting architectural patterns, document both and recommend one with rationale.
- If the user rejects the design proposal, incorporate all feedback and present a revised proposal.
- If the target directory does not exist, create it.
- If operating in update mode, preserve sections the user has not asked to change.
