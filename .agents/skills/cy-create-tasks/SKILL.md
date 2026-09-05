---
name: cy-create-tasks
description: "Break an existing spec into independently shippable tasks with implementation context and single ownership of each test case; enrich existing task files. Excludes spec creation and task execution."
---

# Create Tasks

Decompose an existing requested spec into independently implementable outcomes. Reuse its decisions and file-reference index; a new task does not require starting research from zero.

1. Read `_spec.md`, the accepted graph if present, and applicable `_tests.md`, story/surface contracts, ADRs, and analysis. Inspect additional code only for a missing boundary, dependency, or test owner. A read-only explorer is useful for independent unknowns, not mandatory per task.
2. Choose outcomes and dependencies. Put the earliest useful solution to the motivating problem first. A foundation task names its consumer and verifies its boundary. Size by risk and ownership; file count and a default five-slice quota do not decide the cut.
3. Preserve the runtime manifest contract: `_tasks.md` has `schema_version: "compozy.tasks/v2"`, `workflow`, and `graph.nodes` (`id`, `file`) plus `graph.edges` (`from`, `to`). Dependencies live only there; each edge means its source completes first. Use sequential `task_01` IDs/files and `edges: []` when independent. Keep the table `# | Title | Status | Complexity | Dependencies` consistent with the graph.
4. Each task file has frontmatter `status`, `title`, `type`, `complexity`. Types are free-form lowercase slugs; preserve intentional runtime-rule routing. Complexity is `low|medium|high|critical` and reflects risk.
5. Use the applicable sections of `references/task-template.md` and `references/task-context-schema.md`: Overview, Shippable Outcome, requirements/subtasks, relevant implementation/contract files, deliverables, Tests, and acceptance. Link the owning compatibility/impact analysis instead of duplicating it. Add ADR/competitor references only when relevant, with actual paths.
6. Assign every existing test-contract ID exactly once to the task completing that behavior. Reuse its canonical suite. Missing cases need a concrete invariant/input/result; do not create tests merely to populate a template. A task with existing sufficient coverage records that evidence.
7. For a named visual reference, enumerate the touched states/viewports and required reference/implementation evidence; production content/components/host chrome retain their owners. Ordinary UI edits without a named reference do not acquire Visual Contract Mode.
8. Preserve approval already given. Present a proposed breakdown when product scope/dependencies need a new decision; otherwise generate the requested reviewable files directly. Update complete existing tasks only when their inputs changed.
9. Check graph consistency, test ownership, motivating-outcome coverage, and applicable visual rows once. A requested full `cy-loop-tasks` package keeps the trailing QA pair required by its phase protocol; ordinary fixes need no spec-cycle graph. The pair covers integration gaps and reuses valid slice evidence.

A missing spec or unresolved contract is reported concretely; gather available context before asking. Do not silently drop requested outcomes or test IDs, fabricate unavailable references, or repeat enrichment across already-grounded tasks.
