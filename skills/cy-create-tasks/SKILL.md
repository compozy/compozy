---
name: cy-create-tasks
description: Decomposes PRDs and TechSpecs into detailed, independently implementable task files with enrichment from codebase exploration. Use when a PRD or TechSpec exists and needs to be broken down into executable tasks, or when task files need enrichment with implementation context. Do not use for PRD creation, TechSpec generation, or direct task execution.
argument-hint: "[feature-name] [prd-file]"
---

# Create Tasks

Decompose requirements into detailed, actionable task files with codebase-informed enrichment.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- At minimum, `_prd.md` or `_techspec.md` in that directory.

## Workflow

1. Load type registry.
   - Read `.compozy/config.toml`.
   - If it contains `[tasks].types`, use that list as the allowed `type` values.
   - Otherwise use the built-in defaults: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.

2. Load context.
   - Read `_prd.md` and `_techspec.md` from `.compozy/tasks/<name>/`.
   - Read existing ADRs from `.compozy/tasks/<name>/adrs/` to understand the decision context behind requirements and design choices.
   - If `_techspec.md` is missing:
     - Warn the user that tasks will be higher-level without TechSpec implementation guidance.
     - Derive tasks from PRD functional requirements and user stories instead of TechSpec implementation sections.
     - During enrichment, rely more heavily on codebase exploration to fill `## Implementation Details`, `### Relevant Files`, and `### Dependent Files`.
     - Mark `<requirements>` with PRD-derived behavioral requirements instead of TechSpec-derived technical requirements.
     - Explicitly call out missing implementation detail gaps in the task body instead of inventing specifics.
   - If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
   - Spawn an Agent tool call to explore the codebase for files to create or modify, test patterns, and coding conventions.

3. Break down into tasks.
   - Decompose implementation sections from the TechSpec into granular, independently implementable tasks.
   - **Each task MUST be independently implementable when all of its declared dependencies are met.** No task may require undeclared work from another task. If two tasks share a tight coupling, either merge them or extract the shared piece into a dependency task.
   - **No circular dependencies.** If task A depends on task B, task B must NOT depend on task A (directly or transitively).
   - Each task must have: title, type, complexity, and dependencies.
   - Assign complexity using these criteria:
     - `low`: Single file change, no new interfaces, no concurrency, straightforward logic.
     - `medium`: 2-4 files, may introduce a new interface or struct, limited integration points.
     - `high`: 5+ files, new subsystem or significant refactor, multiple integration points, concurrency involved.
     - `critical`: Cross-cutting change affecting many packages, high risk of regression, requires coordination with other tasks.
   - When a task directly implements or is constrained by a specific ADR, include the ADR reference in the task's "Related ADRs" section under Implementation Details.
   - Embed test requirements in every task. Never create separate tasks dedicated solely to testing.
   - Follow the structure defined in `references/task-template.md`.
   - Refer to `references/task-context-schema.md` for metadata field definitions.

4. Present task breakdown for interactive approval.
   - Show all tasks with: titles, descriptions, complexity ratings, and dependency chains.
   - Wait for user feedback before proceeding.
   - If the user requests changes, revise the breakdown and present again.
   - Iterate until the user explicitly approves.

5. Generate task files.
   - Write `_tasks.md` as the master task list using this exact markdown table format:
     ```markdown
     # [Feature Name] — Task List

     ## Tasks

     | # | Title | Status | Complexity | Dependencies |
     |---|-------|--------|------------|--------------|
     | 01 | [Task title] | pending | [low/medium/high/critical] | [task_NN, ... or —] |
     ```
   - Write individual task files as `task_01.md`, `task_02.md`, through `task_N.md`.
   - Task files use the `task_` prefix without a leading underscore.
   - Each file must start with YAML frontmatter containing `status`, `title`, `type`, `complexity`, and `dependencies`. Use `dependencies: []` when there are no dependencies — do not omit the field.
   - Task numbering must be sequential and consistent between `_tasks.md` and individual files.

6. Enrich each task file.
   - For each task file, check whether it already has `## Overview`, `## Deliverables`, and `## Tests` sections. If all three exist, skip enrichment for that file.
   - Map the task to PRD requirements and TechSpec guidance.
   - Spawn an Agent tool call to discover relevant files, dependent files, integration points, and project rules for this specific task.
   - Fill ALL template sections from `references/task-template.md`. Every task file MUST contain each of the following sections — omitting any is a failure:
     - `## Overview`: what the task accomplishes and why, in 2-3 sentences.
     - `<critical>` block: the standard critical reminders block (read PRD/TechSpec, reference TechSpec, focus on WHAT, minimize code, tests required).
     - `<requirements>` block: specific, numbered technical requirements using MUST/SHOULD language.
     - `## Subtasks`: 3-7 checklist items describing WHAT, not HOW.
     - `## Implementation Details`: file paths to create or modify, integration points. Reference TechSpec for patterns.
     - `### Relevant Files`: discovered paths from codebase exploration with brief reasons.
     - `### Dependent Files`: files that will be affected by this task with brief reasons.
     - `### Related ADRs`: links to relevant ADRs if any exist, or omit subsection if no ADRs apply.
     - `## Deliverables`: concrete outputs with mandatory test items and at least 80% coverage target.
     - `## Tests`: specific test cases as checklists, split into unit tests and integration tests categories.
     - `## Success Criteria`: measurable outcomes including "All tests passing" and "Test coverage >=80%".
   - Reassess complexity based on exploration findings and update if changed.
   - Update the task file in place with enriched content.
   - If enrichment fails for one task, continue to the next and report all failures at the end.

7. Run task validation.
   - Run `compozy tasks validate --name <feature>`.
   - If it exits non-zero, fix the reported issues and re-run.
   - Do not mark the skill complete until it exits 0.

## Anti-Patterns

Do NOT produce tasks with these defects:

- **Mega-tasks.** If a task touches more than 7 files or has more than 7 subtasks, it is too broad. Split it into smaller tasks with explicit dependencies between them.
- **TechSpec duplication.** Do NOT copy interface definitions, code snippets, or architectural diagrams from the TechSpec into task files. Reference the TechSpec section by name (e.g., "See TechSpec 'Core Interfaces' section") instead of reproducing its content.
- **Vague test cases.** Do NOT write test descriptions like "test the happy path" or "verify error handling." Each test case must name the specific input, condition, or behavior being verified (e.g., "POST /job/done with unknown job ID returns 404").

## Error Handling

- If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
- If the user rejects the task breakdown, incorporate all feedback before presenting again.
- If codebase exploration reveals task boundaries that do not match the TechSpec, note the discrepancy and ask the user how to proceed.
- If the target directory does not exist, create it.
- If a task file already exists and is fully enriched, skip it and move to the next.
