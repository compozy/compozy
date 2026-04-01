---
name: create-prd
description: Creates a Product Requirements Document through interactive brainstorming with parallel codebase and web research. Use when starting a new feature or product, building a PRD, or brainstorming requirements. Do not use for technical specifications, task breakdowns, or code implementation.
---

# Create PRD

Create a business-focused Product Requirements Document through structured brainstorming.

## Required Inputs

- Feature name or product idea.
- Optional: existing `_prd.md` file for update mode.

## Workflow

1. Determine the project name and working directory.
   - Derive the slug from the feature name provided by the user.
   - Use `.compozy/tasks/<slug>/` as the target directory.
   - If `_prd.md` already exists in the target directory, read it and operate in update mode.
   - If the directory does not exist, create it.
   - Create `.compozy/tasks/<slug>/adrs/` directory if it does not exist.

2. Discover context through parallel research.
   - Spawn one Agent tool call to explore the codebase for relevant patterns, existing features, and architecture.
   - Spawn a second Agent tool call to perform 3-5 web searches for market trends, competitive analysis, and user needs.
   - Merge findings from both agents before proceeding to questions.

3. Ask clarifying questions following `references/question-protocol.md`.
   - Focus exclusively on WHAT features users need, WHY it provides business value, and WHO the target users are.
   - Ask about success criteria and constraints.
   - Never ask technical implementation questions about databases, APIs, frameworks, or architecture.
   - Ask at most 3 questions per round and wait for answers before the next round.
   - Complete at least one full clarification round before presenting approaches.

4. Present product approaches.
   - Offer 2-3 product approaches with trade-offs for each.
   - Lead with the recommended approach and explain why it is preferred.
   - Wait for the user to select an approach before continuing.
   - After the user selects an approach, create an ADR for this decision:
     - Read `references/adr-template.md`.
     - Determine the next ADR number by listing existing files in `.compozy/tasks/<slug>/adrs/`.
     - Fill the template: the selected approach as "Decision", rejected approaches as "Alternatives Considered" with their trade-offs, and outcomes as "Consequences". Set Status to "Accepted" and Date to today.
     - Write the ADR to `.compozy/tasks/<slug>/adrs/adr-NNN.md` (zero-padded 3-digit number, e.g., `adr-001.md`).

5. Refine the chosen approach.
   - Ask targeted follow-up questions based on the selected approach.
   - Confirm key decisions about scope, phasing, and success criteria with the user before generating the document.
   - If the user makes a significant scope decision during refinement (e.g., including or excluding a major feature, choosing a phasing strategy), create an additional ADR for that decision following the same process as step 4.

6. Generate the PRD document.
   - Read `references/prd-template.md` and fill every section with the gathered context.
   - Include an "Architecture Decision Records" section listing all ADRs created during this session with their numbers, titles, and one-line summaries as links to the `adrs/` directory.
   - Write the completed document to `.compozy/tasks/<slug>/_prd.md`.
   - The PRD must describe user capabilities and business outcomes only.
   - No databases, APIs, code structure, frameworks, testing strategies, or architecture decisions.

## Error Handling

- If the user provides insufficient context to complete a section, note it in the Open Questions section rather than guessing.
- If web research tools are unavailable, proceed with codebase exploration only and note the limitation.
- If the target directory cannot be created, stop and report the filesystem error.
- If operating in update mode, preserve sections the user has not asked to change.
