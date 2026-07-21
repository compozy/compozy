---
name: cy-create-tasks
description: Decomposes PRDs and TechSpecs into robust, independently implementable task files, assigning every test case from _tests.md to exactly one task and enriching tasks from codebase exploration. Use when a PRD or TechSpec exists and needs to be broken down into executable tasks, or when task files need enrichment with implementation context. Do not use for PRD creation, TechSpec generation, or direct task execution.
argument-hint: "[feature-name] [prd-file]"
---

# Create Tasks

Decompose requirements into robust, independently implementable task files with codebase-informed enrichment.

When `_task_groups.md` is present, this skill is also the planning boundary
for optional Task Groups. Read `references/task-group-planning.md` before
offering that branch; it contains proposal state, artifact, ownership, and
verification contracts. The ordinary branch remains unchanged.

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

3. Draft the task breakdown in memory.
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
   - Keep the draft in session memory. Do not create `_tasks.md`, task files, `_task_groups.md`, task group directories, or temporary planning artifacts before the user sees the breakdown and chooses a delivery shape.

4. Assign the test contract.
   - Assign every `UT-`, `IT-`, and `E2E-` ID from `_tests.md` to exactly one task — the task that implements the behavior the case verifies. Integration and E2E cases go to the task that completes the flow they exercise.
   - Done when every ID in `_tests.md` appears in exactly one task's planned `## Tests` section: no orphan IDs, no duplicates.
   - If `_tests.md` is missing: warn the user, then write concrete inline cases per task instead — each naming the exact input, condition, and expected result (e.g., "POST /job/done with unknown job ID returns 404"), never a vague "test the happy path".

5. Present the task breakdown before choosing the delivery shape.
   - Lead with `Proposed task breakdown — N tasks`, where `N` is the complete draft task count.
   - Show every task in one table with provisional `task_NN` ID, title, type, complexity, one-line scope, dependencies, and exact assigned test IDs. When `_tests.md` is absent, show each concrete inline case instead. Counts without task rows or exact assignments are not decision-ready.
   - Accept task edits before asking for a delivery choice. After every edit, revalidate dependencies and exactly-once test assignment, then render the complete table and updated task count again.
   - Only after the complete breakdown is visible, derive concrete decision context for four planning strategies. Keep candidate partitions in session memory and write no artifacts:
     - Count the currently displayed tasks as `N`.
     - Count coherent, reviewable outcome groups as `M`.
     - Count viable groups formed primarily from the registered task `type` values as `K`.
   - Lead the decision prompt with `Choose a delivery shape after any desired task edits:`. Present this menu, replacing `N`, `M`, `K`, and the file range with concrete values:
     - `Ordinary — as drafted`: generate `_tasks.md` and `task_01.md`–`task_N.md` at the initiative root.
     - `Ordinary — smaller tasks`: split the displayed work at the smallest independently implementable boundaries, then show the complete revised breakdown before approval.
     - `Task Groups — by outcome (M groups)`: reorganize the scope into `M` coherent, independently reviewable outcomes, then show a separate Task Group proposal for approval.
     - `Task Groups — by task type (K groups)`: organize tasks primarily by their registered `type`, preserve required cross-type dependency edges, then show a separate Task Group proposal for approval.
   - Mark exactly one feasible strategy `(recommended)` and give one sentence of rationale based on coupling, dependency order, review boundaries, and execution overhead. Default to `Ordinary — as drafted` when no alternative has a clear structural advantage.
   - Show an infeasible strategy as `unavailable` with a concrete reason instead of inventing a count. Task Group strategies require readable canonical PRD and TechSpec files and at least two valid groups. Type-based grouping is unavailable when the draft has fewer than two viable task types or would create tightly coupled, non-reviewable groups.
   - Treat `Ordinary — smaller tasks` as a revision request, not approval. Preserve independent implementability, keep tests with their behavior, revalidate the graph and exactly-once test assignment, render the full revised table, and offer the menu again. If no safe split exists, explain why and retain the current breakdown.
   - Treat either Task Group selection as a partitioning strategy, not approval. Open the editable proposal in step 6 and require its separate confirmation before writing artifacts.
   - Never ask the user to choose Task Groups before showing how many tasks exist and what each task owns.
   - Treat the first task count as decision context, not an immutable partition. Task Group boundaries may require merging or splitting tasks; disclose every changed task and the revised total in the task group proposal before its separate approval.

6. Prepare and confirm the Task Group proposal when selected.
   - Require readable canonical `_prd.md` and `_techspec.md`. Missing or unreadable PRD/TechSpec stops Task Group readiness and names that canonical path; create no marker or task group directory.
   - Recommend `ordinary` when the displayed implementation map has no coherent separable outcome, including zero candidates or one inseparable component. Explain the recommendation; never render an empty executable Task Group marker.
   - For `Task Groups — by outcome`, cluster tasks around user-visible or architectural outcomes that can be implemented and reviewed as meaningful slices. Use dependency edges for prerequisite outcomes rather than grouping by file location.
   - For `Task Groups — by task type`, cluster tasks primarily by their registered `type`. Keep each task intact, retain cross-type dependencies as Task Group dependency edges with rationale, and reject the partition when it would turn tightly coupled work into non-reviewable layers.
   - Load an existing valid plan into edit mode rather than replacing it. Keep proposal edits, stable IDs, physical directories, completion states, task-group-local task graphs, exact test assignments, and source checksum in session memory until confirmation.
   - Present every proposed task group using the approval format in `references/task-group-planning.md`. Each task group MUST include its complete task table with qualified task ID, title, type, complexity, one-line scope, task dependencies, and exact assigned test IDs. Task Group-level counts without task rows are not approval-ready.
   - Before confirmation, allow add/remove/split/combine/rename/reorder, task group dependency, and task-breakdown edits. Revalidate titles, outcomes, scopes, stable IDs, dependency targets, cycles, affected edges, task ownership, and test assignment after edits, then render the complete proposal and revised total task count again. Keep every task group, task row, and rationale keyboard- and screen-reader-navigable, including proposals larger than one screen.
   - On cancel, write nothing: a first proposal leaves no marker, task group directory, task file, or temporary artifact; an existing plan retains its last confirmed bytes and task group directories. A stale confirmation must fail rather than overwrite a newer confirmed plan.

7. Generate the approved task artifacts.
   - In `ordinary` mode, write the approved tasks at the initiative root and create no Task Group marker or task group directory.
   - In Task Group mode, use the artifact, path, and state rules in `references/task-group-planning.md`, `references/task-template.md`, and `references/task-context-schema.md`.
   - For new Task Group plans, derive each physical directory as `_task_groups/NNN-<brief>/`, where `NNN` matches the stable `TG-NNN` ID and `<brief>` is a lowercase kebab-case summary of the approved title. Keep public references as `<initiative>/TG-NNN`. Preserve a persisted directory during later edits, including legacy `_task_groups/TG-NNN/` directories.
   - Stage `_task_groups.md`, each manifest-declared task group directory and `_tasks.md`, and every task group task suite from the confirmed in-memory proposal. Publish them only as one validated generation boundary; a write, validation, or permission failure never reports success or leaves a new executable partial plan.
   - Keep initiative PRD, TechSpec, stories, tests, and ADRs at root. Reference them from task group tasks; never copy specification corpora into task groups.
   - Validate every generated task group manifest and run the initiative-wide qualified ownership and exactly-once `_tests.md` assignment audit before reporting success. Repeated `task_01` names are valid only when qualified by different task group IDs.

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

8. Enrich each task file.
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

9. Validate.
   - Run `compozy tasks validate --name <feature>`. If it exits non-zero, fix the reported issues and re-run; do not finish until it exits 0.
   - Audit the test assignment: every ID in `_tests.md` appears in exactly one task file's `## Tests` section. Fix any orphan or duplicate and re-audit.
   - In Task Group mode, execute every scenario in `references/task-group-planning.md` and record its observable result. A green parser check alone does not prove cancellation, stale-write safety, atomic generation, ownership, or accessible large-proposal behavior.

10. Preserve explicit lifecycle boundaries.
   - Task creation ends after generation and validation. It does not execute a task group, select a branch, run review, invoke completion, or advance to another task group.
   - Task Group execution uses the user's current branch/worktree and existing explicit commands. Independent task groups are informationally eligible, never auto-started; ordinary workflows retain their current paths and behavior.

## Error Handling

- If both `_prd.md` and `_techspec.md` are missing, stop and ask the user to create at least one first.
- If the user rejects the task breakdown, incorporate all feedback before presenting again.
- If the user cancels before choosing or confirming a delivery shape, discard the in-memory draft and write nothing.
- If codebase exploration reveals task boundaries that do not match the TechSpec, note the discrepancy and ask the user how to proceed.
- If a test case in `_tests.md` fits no task, the breakdown is missing a slice — fix the breakdown rather than dropping the case.
- If the target directory does not exist, create it.
- If a task file already exists and is fully enriched, skip it and move to the next.
