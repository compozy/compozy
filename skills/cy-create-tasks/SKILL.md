---
name: cy-create-tasks
description: Decomposes PRDs and TechSpecs into robust, independently implementable task files, assigning every test case from _tests.md to exactly one task and enriching tasks from codebase exploration. Use when a PRD or TechSpec exists and needs to be broken down into executable tasks, or when task files need enrichment with implementation context. Do not use for PRD creation, TechSpec generation, or direct task execution.
argument-hint: "[feature-name] [prd-file]"
---

# Create Tasks

Decompose requirements into robust, independently implementable task files with codebase-informed enrichment.

## Task Sizing

Every task becomes one full agent run: a fresh context that re-reads the spec corpus, re-explores the codebase, and rebuilds its model of the system from zero before the first edit. That ramp-up is the expensive part of a run — many small tasks pay it over and over and discard the accumulated reasoning at every boundary, while a robust task keeps it working.

- Default to fewer, larger tasks. A task is a complete vertical slice — implementation, wiring, and its assigned tests — delivered end-to-end in one run.
- Split only at real boundaries:
  - **Dependency**: a contract (schema, interface, protocol) must exist before its consumers can build on it.
  - **Parallelization**: two slices touch disjoint files and can run as parallel waves via `_tasks.md` edges.
  - **Domain**: different toolchains or deliverables (backend vs frontend vs SDK vs docs).
- File count is never a split reason: a task spanning 20+ files is healthy when they form one coherent slice, and one agent run handles it comfortably.
- A typical feature lands at 3-7 robust tasks. A breakdown with 10+ tasks almost always contains slices that belong together — merge them before presenting.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- At minimum, `_prd.md` or `_techspec.md` in that directory.
- When present: `_tests.md` (test contract) and `_user_stories.md` (story catalog).

## Workflow

1. Load type registry.
   - Read `.compozy/config.toml`.
   - If it contains `[tasks].types`, use that list as the allowed `type` values.
   - Otherwise use the built-in defaults: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.

2. Load context.
   - Read `_prd.md`, `_techspec.md`, `_user_stories.md`, and `_tests.md` from `.compozy/tasks/<name>/`.
   - Read existing ADRs from `.compozy/tasks/<name>/adrs/` to understand the decision context behind requirements and design choices.
   - If `_techspec.md` is missing:
     - Warn the user that tasks will be higher-level without TechSpec implementation guidance.
     - Derive tasks from PRD functional requirements and the `_user_stories.md` catalog instead of TechSpec implementation sections.
     - During enrichment, rely more heavily on codebase exploration to fill `## Implementation Details`, `### Relevant Files`, and `### Dependent Files`.
     - Mark `<requirements>` with PRD-derived behavioral requirements instead of TechSpec-derived technical requirements.
     - Explicitly call out missing implementation detail gaps in the task body instead of inventing specifics.
   - If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
   - Spawn an Agent tool call to explore the codebase for files to create or modify, test patterns, and coding conventions.

3. Break down into tasks.
   - Apply the Task Sizing doctrine above: slice the TechSpec's Build Order into the smallest number of robust tasks the real boundaries allow.
   - **Each task MUST be independently implementable when all dependencies declared in `_tasks.md` graph edges are met.** No task may require undeclared work from another task. If two tasks share a tight coupling, merge them — or extract the shared piece into a dependency task only when a real boundary separates it.
   - **No circular dependencies.** If task A depends on task B, task B must NOT depend on task A (directly or transitively).
   - Each task must have: title, type, complexity, and dependency relationships in the graph plan.
   - Complexity rates implementation risk, not size — and is never a reason to split:
     - `low`: contained change on well-trodden patterns, low regression risk.
     - `medium`: new interfaces or integration points, moderate coordination.
     - `high`: new subsystem, concurrency, or a broad integration surface.
     - `critical`: cross-cutting change with high regression risk, requires coordination with other tasks.
   - When a task directly implements or is constrained by a specific ADR, include the ADR reference in the task's "Related ADRs" section under Implementation Details.
   - Tests live inside the task that implements the behavior they verify; never create tasks dedicated solely to testing.
   - Follow the structure defined in `references/task-template.md` and the metadata definitions in `references/task-context-schema.md`.

4. Assign the test contract.
   - Assign every `UT-`, `IT-`, and `E2E-` ID from `_tests.md` to exactly one task — the task that implements the behavior the case verifies. Integration and E2E cases go to the task that completes the flow they exercise.
   - Done when every ID in `_tests.md` appears in exactly one task's planned `## Tests` section: no orphan IDs, no duplicates.
   - If `_tests.md` is missing: warn the user, then write concrete inline cases per task instead — each naming the exact input, condition, and expected result (e.g., "POST /job/done with unknown job ID returns 404"), never a vague "test the happy path".

5. Present the task breakdown for interactive approval.
   - Show every task with: title, type, complexity, a one-line scope summary, dependency chains, and assigned test-ID counts.
   - Wait for user feedback before proceeding; revise and present again until the user explicitly approves.

6. Generate task files.
   - Write `_tasks.md` as the canonical task graph manifest. It MUST start with this YAML frontmatter shape:
     ```markdown
     ---
     schema_version: "compozy.tasks/v2"
     workflow: [feature-name]
     graph:
       nodes:
         - id: task_01
           file: task_01.md
       edges:
         - from: task_01
           to: task_02
     ---

     # [Feature Name] Task List
     ```
   - `_tasks.md` is the only place dependency relationships are stored. Each edge means `from` must finish before `to` can start.
   - Include every task in `graph.nodes`, using canonical sequential ids (`task_01`, `task_02`, ...) and matching files (`task_01.md`, `task_02.md`, ...).
   - Use `edges: []` when there are no dependencies.
   - Write individual task files as `task_01.md` through `task_N.md` (the `task_` prefix has no leading underscore).
   - Each file must start with YAML frontmatter containing only task-owned metadata: `status`, `title`, `type`, and `complexity`. Dependency information lives only in `_tasks.md`.
   - Task numbering must be sequential and consistent between `_tasks.md` and individual files.

7. Enrich each task file.
   - For each task file, check whether it already has `## Overview`, `## Deliverables`, and `## Tests` sections. If all three exist, skip enrichment for that file.
   - Map the task to PRD requirements, user stories, and TechSpec guidance.
   - Spawn an Agent tool call to discover relevant files, dependent files, integration points, and project rules for this specific task.
   - Fill ALL template sections from `references/task-template.md`. Every task file MUST contain each of the following sections — omitting any is a failure:
     - `## Overview`: what slice of the system the task delivers and why, in 2-3 sentences.
     - `<critical>` block: the standard critical reminders block from the template.
     - `<requirements>` block: specific, numbered technical requirements using MUST/SHOULD language.
     - `## Subtasks`: checklist items describing WHAT, not HOW — one per coherent unit of work, typically 5-12 for a robust task.
     - `## Implementation Details`: file paths to create or modify, integration points. Reference TechSpec for patterns.
     - `### Relevant Files`: discovered paths from codebase exploration with brief reasons.
     - `### Dependent Files`: files that will be affected by this task with brief reasons.
     - `### Related ADRs`: links to relevant ADRs if any exist, or omit the subsection if none apply.
     - `## Deliverables`: concrete outputs, including every assigned test case implemented and passing.
     - `## Tests`: the assigned test-case IDs grouped by level with the behavior they cover; full case definitions stay in `_tests.md`.
     - `## Success Criteria`: measurable outcomes including "Every assigned test case implemented and passing".
   - Reassess complexity based on exploration findings and update if changed.
   - Update the task file in place with enriched content.
   - If enrichment fails for one task, continue to the next and report all failures at the end.

8. Validate.
   - Run `compozy tasks validate --name <feature>`. If it exits non-zero, fix the reported issues and re-run; do not finish until it exits 0.
   - Audit the test assignment: every ID in `_tests.md` appears in exactly one task file's `## Tests` section. Fix any orphan or duplicate and re-audit.

## Error Handling

- If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
- If the user rejects the task breakdown, incorporate all feedback before presenting again.
- If codebase exploration reveals task boundaries that do not match the TechSpec, note the discrepancy and ask the user how to proceed.
- If a test case in `_tests.md` fits no task, the breakdown is missing a slice — fix the breakdown rather than dropping the case.
- If the target directory does not exist, create it.
- If a task file already exists and is fully enriched, skip it and move to the next.
