---
name: cy-tasks-tail-qa-pair
description: "Add or repair the final QA planning/execution pair in a requested cy-loop-tasks graph."
trigger: explicit
---

# Tasks Tail QA Pair

Complete the QA tail of the supplied full-loop task graph. Use the slug/path from
the caller or current task context; modification time does not identify the user's
intended workflow. Ordinary task lists outside a requested full loop need no pair.

Read `_tasks.md` and any existing QA task files. Reuse a valid pair; repair only a
missing half or a concrete wiring/coverage gap, preserving customized task bodies
and unrelated graph entries. Use the
[task metadata schema](../cy-create-tasks/references/task-context-schema.md)
for the runtime metadata/graph contract when needed.

Use `references/qa-tail-template.md` for exact types and applicable body guidance.
Assign the pair remaining changed/integration journeys and final visual rows, with
current task evidence reused. Select real browser/runtime checks by actual behavior;
full suites/labs require scope, risk, or policy. Planning needs no mandatory worker
or runtime. Execution with no remaining checks records its evidence disposition
without inventing sessions or a dated run report.

The result contains `qa-report` followed by `qa-execution`, with sequential task IDs
and risk-based complexity. Add their files and graph nodes/edges together and keep
the table consistent. QA planning follows all implementation prerequisites; in a
linear graph this is one dependency on the last implementation task. Execution
depends on planning. Avoid redundant transitive edges and cycles.

Check the changed graph, task metadata, evidence ownership, and references once.
`references/qa-pair-checklist.md` is a diagnostic aid for an ambiguous existing pair,
not another mandatory audit. Update a QA range in `MVP Boundary` only if present.
Summarize the added/repaired tasks or report that the pair was already valid.
