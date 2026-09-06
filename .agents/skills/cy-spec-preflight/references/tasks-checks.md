# Task Package Checks

Check the current authoring output once and reuse unchanged results:

- `compozy.tasks/v2` metadata, sequential nodes/files, acyclic dependency edges, and the display table agree. Per-task status lives in task frontmatter; `_tasks.md` owns topology.
- Each task names its outcome, relevant contracts/files, task-owned checks, remaining integration checks and owner, deliverables, and acceptance. The earliest useful outcome addresses the motivating problem; foundations identify their consumers.
- Every assigned test ID has exactly one owner and uses a suitable canonical suite. Distinct risks determine cases; there is no category/count quota. A task that changes many behaviors and carries one or two cases records why the remaining behaviors are owned elsewhere (L-011); proportion is judged per invariant, not by count.
- Changed public/config/extension/workspace surfaces link to the owning impact/compatibility analysis. Unaffected facts do not need repeated subsections.
- Named visual references map touched states/viewports to evidence and its owner. A loop can assign complete bundles to integration QA with representative inspection during implementation; explicit task-owned visual acceptance remains required. Unreferenced UI does not automatically require reference parity.
- A full `cy-loop-tasks` graph retains the QA pair required by that phase protocol; it covers remaining changed/integration journeys once, with appropriate browser or CLI/API evidence. Full suites/labs need changed-contract or release scope, not merely UI file presence. Routine tasks do not require this graph.
- Referenced ADRs/competitor paths are concrete and relevant. Required contracts resolve; unknown product decisions are explicit.

Repair actual dependency, contract, or coverage gaps before execution. Formatting keywords and unused template sections are not independent blockers.
