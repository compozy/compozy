---
name: cy-create-tasks
description: Decomposes Specs into shippable slices — robust, independently implementable task files each delivering an outcome observable from outside the system — assigning every test case from _tests.md to exactly one task and enriching tasks from codebase exploration. Use when a spec exists and needs to be broken down into executable tasks, or when task files need enrichment with implementation context. Do not use for spec creation or direct task execution.
---

# Create Tasks

Decompose requirements into robust, independently implementable task files with codebase-informed enrichment.

## Task Sizing

Every task becomes one full agent run: a fresh context that re-reads the spec corpus, re-explores the codebase, and rebuilds its model of the system from zero before the first edit. That ramp-up is the expensive part of a run — many small tasks pay it over and over and discard the accumulated reasoning at every boundary, while a robust task keeps it working.

- A task is a **shippable slice**: the smallest increment that could merge to `main` on its own — implementation, wiring, surfaces, UI, docs, and its assigned tests together — with an outcome observable from outside the system (a user action that newly works, a CLI/API call that newly answers, a screen that newly renders through its real entry path). A slice crosses every layer its outcome needs; a layer grouping ("all backend", "all frontend", "all docs") is an invalid breakdown.
- **Slice 1 solves the spec's Motivating Problem end-to-end** in its simplest honest form; later slices extend it. Order slices by user value — the spec's Build Order informs dependency edges, never sequence.
- Split only at real boundaries:
  - **Dependency**: a contract (schema, interface, protocol) must exist before its consumers can build on it. A foundation-only task is valid only when it names the shippable slice that consumes it, and that consumer is its immediate successor in the graph.
  - **Parallelization**: two slices touch disjoint files and can run as parallel waves via `_tasks.md` edges.
- File count is never a split reason: a task spanning 20+ files is healthy when they form one shippable slice, and one agent run handles it comfortably.
- The slice budget defaults to 5 shippable slices (QA tail excluded) and is overridable per invocation (`slice_budget: N`). When an honest breakdown exceeds the budget, present the overflow as a sequenced program of follow-up specs at approval time — the user chooses between one oversized spec and the program.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- At minimum, `_spec.md` in that directory.
- When present: `_tests.md` (test contract), `_user_stories.md` (story catalog), `_dx.md` (developer-experience contract), and `_uiux.md` (UI change map).

## Workflow

1. Choose the task taxonomy.
   - Prefer the standard work-type slugs: `feature` (the default for shippable slices), `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`, `qa-report`, and `qa-execution`. Layer slugs (`frontend`, `backend`) fit only single-layer foundation slices; when slices span layers, review any `loops.defaults.delivery.runtime_rules[].match.type` routing that assumed layer tasks.
   - When the specification needs a distinct category, define one concise lowercase hyphenated slug in the proposed breakdown and use it consistently. Task `type` is free-form; ordered `loops.defaults.delivery.runtime_rules[].match.type` entries own type-based runtime routing.

2. Load context.
   - Read `_spec.md`, `_user_stories.md`, `_dx.md`, `_uiux.md` (when present), and `_tests.md` from `.compozy/tasks/<name>/`.
   - Read existing ADRs from `.compozy/tasks/<name>/adrs/` to understand the decision context behind requirements and design choices.
   - Resolve every local artifact path or glob cited as a contract, including repo-relative and absolute paths outside the task directory. Read textual contracts and use `eng-ui-screenshot` to render and inspect named visual artifacts; a path mention is an input, not optional background.
   - If `_spec.md` lacks a Technical part (Part II):
     - Warn the user that tasks will be higher-level without implementation guidance.
     - Derive tasks from Part I functional requirements and the `_user_stories.md` catalog instead of Part II implementation sections.
     - During enrichment, rely more heavily on codebase exploration to fill `## Implementation Details`, `### Relevant Files`, and `### Dependent Files`.
     - Mark `<requirements>` with behavior-derived requirements instead of design-derived technical requirements.
     - Explicitly call out missing implementation detail gaps in the task body instead of inventing specifics.
   - If `_spec.md` is missing, stop and ask the user to create it first via `cy-create-spec`.
   - Spawn an Agent tool call to explore the codebase for files to create or modify, test patterns, and coding conventions.

3. Break down into tasks.
   - Apply the Task Sizing doctrine above: cut the spec into shippable slices ordered by user value — the smallest number of robust tasks the real boundaries allow, slice 1 solving the Motivating Problem.
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
   - For every task whose slice touches a surface mapped in `_uiux.md`, define an explicit Visual Contract: one row per touched artboard section, reference state, and required viewport, with the implementation target, fidelity, and source-authorized differences. The `_uiux.md` surface inventory is the coverage floor — derive rows from it, never from what the task happens to cite. Never use “all states,” “match the mock,” or “screenshot parity” as a substitute for enumerating rows.
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
   - For each task file, check whether it already has `## Overview`, `## Shippable Outcome`, `## Deliverables`, and `## Tests` sections, plus a complete `## Visual Contract` when its slice touches a surface mapped in `_uiux.md`. Skip enrichment only when every required and conditional section is complete.
   - Map the task to Part I requirements, user stories, Part II guidance, the `_dx.md`/`_uiux.md` surface contracts, and its subset of the `_spec.md` File References index.
   - Spawn an Agent tool call to discover relevant files, dependent files, integration points, and project rules for this specific task.
   - Fill ALL template sections from `references/task-template.md`. Every task file MUST contain each of the following sections — omitting any is a failure:
     - `## Overview`: what a user, agent, or operator can newly do when this task merges, and why it matters, in 2-3 sentences.
     - `## Shippable Outcome`: the observable outcome plus its verification tier — the cheapest check that can falsify it: `gate` (the slice's tests and lints prove it) | `probe` (a named CLI/HTTP/UDS call shows it) | `smoke` (open the surface through its real entry path and capture the touched Visual Contract sections).
     - `<critical>` block: the standard critical reminders block from the template.
     - `<requirements>` block: specific, numbered technical requirements using MUST/SHOULD language.
     - `## Subtasks`: checklist items describing WHAT, not HOW — one per coherent unit of work, typically 5-12 for a robust task.
     - `## Implementation Details`: file paths to create or modify, integration points. Reference `_spec.md` Part II for patterns.
     - `### Relevant Files`: discovered paths from codebase exploration with brief reasons.
     - `### Dependent Files`: files that will be affected by this task with brief reasons.
     - `### Competitor References`: this task's `.resources/<repo>/path` entries copied from `_spec.md` File References — same paths, never paraphrased; omit the subsection when the spec cites none.
     - `### Related ADRs`: links to relevant ADRs if any exist, or omit the subsection if none apply.
     - `## Deliverables`: concrete outputs, including every assigned test case implemented and passing.
     - `## Tests`: the assigned test-case IDs grouped by level with the behavior they cover; full case definitions stay in `_tests.md`.
     - `## Success Criteria`: measurable outcomes including "Every assigned test case implemented and passing".
   - When the task's slice touches a surface mapped in `_uiux.md`, also include `## Visual Contract` from the template. Its deliverables and success criteria MUST require the durable `eng-ui-screenshot` evidence bundle for every row; implementation-only captures are invalid evidence.
   - Reassess complexity based on exploration findings and update if changed.
   - Update the task file in place with enriched content.
   - If enrichment fails for one task, continue to the next and report all failures at the end.

8. Audit the resulting task package.
   - Audit the test assignment: every ID in `_tests.md` appears in exactly one task file's `## Tests` section. Fix any orphan or duplicate and re-audit.
   - Audit mission traceability: exactly one slice's `## Shippable Outcome` names the spec's Motivating Problem as solved end-to-end, and it is the earliest slice the dependency graph allows. A breakdown where no slice solves it — or only the last one does — is invalid; fix the breakdown.
   - Audit visual coverage against the `_uiux.md` inventory: every artboard section of every mapped surface touched by any slice appears in that slice's Visual Contract, and each row names the durable evidence bundle. Fix vague or missing rows before finishing.

## Error Handling

- If `_spec.md` is missing, stop and ask the user to create it first via `cy-create-spec`.
- If the user rejects the task breakdown, incorporate all feedback before presenting again.
- If codebase exploration reveals task boundaries that do not match the spec, note the discrepancy and ask the user how to proceed.
- If a test case in `_tests.md` fits no task, the breakdown is missing a slice — fix the breakdown rather than dropping the case.
- If the target directory does not exist, create it.
- If a task file already exists and is fully enriched, skip it and move to the next.
- If a named visual reference is missing or cannot be rendered, stop before generating the affected task; do not downgrade it to prose guidance.
