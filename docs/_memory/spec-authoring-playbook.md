# Spec Authoring Playbook

Use the current phase and changed contract to select guidance. This playbook coordinates authoring; it does not require a separate full read/research pass for each task or checkpoint.

## Shared Decisions

- State the motivating problem, observable outcome, scope, and non-goals. Preserve accepted user goals; record user decisions that narrow them.
- Apply SD-013 to compatibility: user state migrates losslessly, public surfaces deprecate before deletion except documented experimental contracts, and internals hard-cut. List delete targets and regimes where breaking changes occur. L-040 explains the current policy.
- Record extensibility, agent operations, config lifecycle, workspace isolation, and Web/Docs impact once at the owning artifact (`change-impact.md`). Consumers link to it and update only affected entries.
- Reuse approved research, current code findings, and relevant ADRs. Consult competitors only when the task needs that evidence; record actual source paths. Market research is conditional, not a default cost for every spec.

## Product and Surface Design

Write WHAT/WHY/WHO with verifiable acceptance. Technical terms may remain when they are the public contract or an explicit constraint; incidental implementation goes in the technical part (L-013).

Use `_user_stories.md` for distinct journeys. `_dx.md` records agent/operator interactions; an internal-only change may state no public surface. UI-bearing features use `_uiux.md` to map changed surfaces, production owners, and any named visual reference. Do not invent prototype content, controls, or new primitives to match a reference.

Ask only for unresolved product choices after available evidence is used. Approved direction satisfies earlier checkpoints. ADRs record consequential alternatives, not every ordinary implementation decision. Save the spec before offering optional peer review; apply only user-selected findings.

## Technical Design

Always make the outcome and architectural boundaries clear. Other markers are conditional (L-012):

| Change | Required design detail |
| --- | --- |
| Interface/API | Concrete changed signatures/schema and consumers |
| Persistent/config data | Field purpose, ownership, defaults, and SD-013 upgrade plan |
| Queryable metadata/state | Side-table/JSON choice where there is a real ownership/query trade-off |
| Concurrency, ownership, permissions | Applicable invariants and failure behavior |
| Public/runtime surface | Agent/extension/config/Web/Docs impact and compatibility |

A missing applicable contract is a design gap. An irrelevant marker needs at most a short no-impact entry, never invented interfaces, side tables, or lease rules. Templates and heuristic keyword checks support this review; they do not override semantics.

File References names the relevant contract/code/competitor paths and why each matters. `_tests.md` defines concrete cases with canonical suite ownership; existing coverage or stronger gates may already satisfy an invariant. Testing strategy belongs in the spec, full cases in the catalog, assigned IDs in tasks.

## Task Generation

Preserve the `compozy.tasks/v2` graph/metadata contract from `cy-create-tasks`. Cut by useful outcomes and dependencies; do not force a fixed number of slices or repeat research for each task. A foundation task names its consumer and validates its own boundary.

Assign every test ID exactly once to the task completing its behavior. No happy/failure/concurrency quota: add cases for distinct risks. For named visual references, enumerate the affected states/viewports and their evidence owner. Reuse shared impact and compatibility analysis through links.

A requested full `cy-loop-tasks` package keeps the QA pair required by its phase protocol; that phase verifies remaining integration journeys once. Ordinary fixes can use focused tests/probes without adopting the full spec-cycle pipeline.

## Execution and Memory

Task files point to the canonical artifacts their executor must read. Reuse a current inventory; expand only for stale, missing, or conflicting dependencies. Record meaningful decisions and handoff evidence for long tasks in the existing memory files. Do not print an additional full checklist, duplicate memory, or re-audit unchanged contracts at every checkpoint.

Use the root's scoped verification/delivery rules and the task's changed-behavior evidence. L-029 teardown applies whenever a lab is created. Historical synthesis and incident counts explain these decisions; they are not a second executable backlog.
