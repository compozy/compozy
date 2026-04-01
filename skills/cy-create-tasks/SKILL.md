---
name: cy-create-tasks
description: Decomposes PRDs and TechSpecs into detailed, independently implementable task files with enrichment from codebase exploration. Use when a PRD or TechSpec exists and needs to be broken down into executable tasks, or when task files need enrichment with implementation context. Do not use for PRD creation, TechSpec generation, or direct task execution.
---

# Create Tasks

Decompose requirements into detailed, actionable task files with codebase-informed enrichment.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- At minimum, `_prd.md` or `_techspec.md` in that directory.

## Workflow

1. Load context.
   - Read `_prd.md` and `_techspec.md` from `.compozy/tasks/<name>/`.
   - Read existing ADRs from `.compozy/tasks/<name>/adrs/` to understand the decision context behind requirements and design choices.
   - Warn the user if `_techspec.md` is missing but continue with available documents.
   - If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
   - Spawn an Agent tool call to explore the codebase for files to create or modify, test patterns, and coding conventions.

2. Break down into tasks.
   - Decompose implementation sections from the TechSpec into granular, independently implementable tasks.
   - Each task must have: title, domain, type, scope, complexity, and dependencies.
   - When a task directly implements or is constrained by a specific ADR, include the ADR reference in the task's "Related ADRs" section under Implementation Details.
   - Embed test requirements in every task. Never create separate tasks dedicated solely to testing.
   - Follow the structure defined in `references/task-template.md`.
   - Refer to `references/task-context-schema.md` for metadata field definitions.

3. Present task breakdown for interactive approval.
   - Show all tasks with: titles, descriptions, complexity ratings, and dependency chains.
   - Wait for user feedback before proceeding.
   - If the user requests changes, revise the breakdown and present again.
   - Iterate until the user explicitly approves.

4. Generate task files.
   - Write `_tasks.md` as the master task list containing all task titles, statuses, and dependencies.
   - Write individual task files as `task_01.md`, `task_02.md`, through `task_N.md`.
   - Task files use the `task_` prefix without a leading underscore.
   - Each file must start with YAML frontmatter containing `status`, `domain`, `type`, `scope`, `complexity`, and `dependencies`.
   - Task numbering must be sequential and consistent between `_tasks.md` and individual files.

5. Enrich each task file.
   - For each task file, check whether it already has `## Overview`, `## Deliverables`, and `## Tests` sections. If all three exist, skip enrichment for that file.
   - Map the task to PRD requirements and TechSpec guidance.
   - Spawn an Agent tool call to discover relevant files, dependent files, integration points, and project rules for this specific task.
   - Fill all template sections from `references/task-template.md`:
     - Overview: what the task accomplishes and why, in 2-3 sentences.
     - Requirements: specific, numbered technical requirements.
     - Subtasks: 3-7 checklist items describing WHAT, not HOW.
     - Implementation Details: file paths to create or modify, integration points. Reference TechSpec for patterns.
     - Relevant Files and Dependent Files: discovered paths from codebase exploration.
     - Deliverables: concrete outputs with mandatory test items and at least 80% coverage target.
     - Tests: specific test cases as checklists for unit and integration tests.
     - Success Criteria: measurable outcomes.
   - Reassess complexity based on exploration findings and update if changed.
   - Update the task file in place with enriched content.
   - If enrichment fails for one task, continue to the next and report all failures at the end.

## Error Handling

- If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
- If the user rejects the task breakdown, incorporate all feedback before presenting again.
- If codebase exploration reveals scope that does not match the TechSpec, note the discrepancy and ask the user how to proceed.
- If the target directory does not exist, create it.
- If a task file already exists and is fully enriched, skip it and move to the next.
