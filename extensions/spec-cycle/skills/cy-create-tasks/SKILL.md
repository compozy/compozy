---
name: cy-create-tasks
description: "Create or refine a task graph from an existing Compozy spec, with outcome, dependency, and test ownership."
---

# Create Tasks

Decompose an existing requested spec into independently implementable outcomes. Reuse its decisions and file-reference index; a new task does not require starting research from zero.

1. Start from the requested scope and `_spec.md` outcome/reference index. Reuse the accepted graph; read only the assigned test definitions and relevant surface/ADR sections needed to establish a task boundary or resolve a dependency. Expand into code when an ownership or contract question remains.
2. Choose outcomes and dependencies. Put the earliest useful solution to the motivating problem first. A foundation task names its consumer and verifies its boundary. Resolve a shared contract before dispatching its consumers, or keep the coupled work in one task. Size by risk and ownership; file count and a default five-slice quota do not decide the cut.
3. Use `references/task-context-schema.md` when creating or changing metadata/graph structure. `_tasks.md` owns dependencies; individual frontmatter owns task status/type/complexity. Preserve runtime routing and keep the display table consistent with the graph.
4. Use `references/task-template.md` for the task body. Include its outcome, relevant constraints/files, checks, and acceptance; omit empty or inapplicable sections. Link the owning compatibility/impact analysis and applicable ADR/external references instead of copying them.
5. Assign every existing test-contract ID exactly once to the task completing that behavior. Reuse its canonical suite. Missing cases need a concrete invariant/input/result; do not create tests merely to populate a template. A task with existing sufficient coverage records that evidence.
6. Assign task checks and remaining integration checks to one owner each. For named visual references, use the template's Visual Contract rows and evidence owner; ordinary UI edits do not acquire reference parity. A full `cy-loop-tasks` graph retains its QA pair for remaining journeys/visual rows once, with current evidence reused. Full suites/labs follow actual scope or policy.
7. Generate the requested files and check graph consistency, test ownership, outcome coverage, and applicable visual rows once. Preserve approval already given; a new decision is needed only when an unresolved scope/dependency choice blocks a correct graph. Update completed tasks only when their inputs changed.

A missing spec or unresolved contract is reported concretely; gather available context before asking. Do not silently drop requested outcomes or test IDs, fabricate unavailable references, or repeat enrichment across already-grounded tasks.
