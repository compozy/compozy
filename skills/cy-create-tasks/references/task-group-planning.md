# Task Group Planning Contract

Use this reference when maintainer chooses Task Group planning. It is opt-in
planning layer over one canonical PRD and TechSpec, not delivery orchestrator.

## Preconditions and recommendation

1. Enter this contract only after `cy-create-tasks` has rendered the complete
   neutral task table and total task count. The user must know the proposed
   tasks, ownership, dependencies, and exact test assignments before choosing a
   delivery shape.
2. Read and parse `.compozy/tasks/<initiative>/_prd.md` and
   `.compozy/tasks/<initiative>/_techspec.md` completely. Verify both readable
   and canonical before offering Task Groups.
3. Enter this contract after the user selects either Task Group partitioning
   strategy from the delivery-shape menu. The selection chooses how to draft the
   proposal; it neither approves boundaries nor writes artifacts. Either
   ordinary strategy preserves the existing task directory, `_tasks.md`, task
   names, commands, and behavior and creates no `_task_groups.md` or
   `_task_groups/`.
4. Recommend `ordinary` when the displayed implementation map has zero nodes or one
   inseparable component. Render non-empty rationale and
   `ordinary_recommended`; never render executable zero-node proposal.
5. Recommend Task Groups only for coherent, reviewable partitions. Outcome-based
   planning clusters user-visible or architectural outcomes. Type-based planning
   clusters primarily by registered task `type`, retains cross-type dependency
   edges and rationales, and is unavailable when it would create tightly coupled,
   non-reviewable layers. Explain for each candidate stable ID, title, outcome,
   owned scope, dependencies, dependency rationale, and independent peers.
   File/task/line counts warn; they do not enforce boundaries.

## Editable proposal state

Keep proposal state in current session until confirmation. State includes source
checksum, stable `TG-NNN` IDs, physical directory, title, outcome, owned scope,
dependency edges and rationales, independent groups, current checkbox state,
task-group-local task graphs, and exact test assignments.

Existing valid `_task_groups.md` is an edit session: load it, preserve IDs,
physical directories, and `[ ]`/`[x]` states, and show current checksum. Do not
silently replace it or create duplicate task group directories. Proposal may add,
remove, split, combine, rename, reorder, edit task group or task edges, and revise
task ownership/test assignments. Removing a referenced task group shows every
affected dependent edge before edit can be confirmed. Blank title/outcome,
unknown/self dependencies, cycles, duplicate IDs, incomplete task tables, and
ownership conflicts block confirmation.

## Approval presentation contract

The neutral task preview and four-strategy delivery menu are the first decision
surface. Selecting either Task Group strategy opens this second approval surface;
it does not approve task group boundaries or write artifacts. If task group
design merges or splits neutral tasks, list every change and the revised total
task count before asking for confirmation.

Approval covers both task group boundaries and the tasks inside them. Render every
task group in dependency order with its directory, outcome, owned scope, task group
dependencies and rationales, independent peers, and test totals. Immediately
under each task group, render the complete proposed task table:

| Task | Title | Type | Complexity | Scope | Depends on | Assigned tests |
| --- | --- | --- | --- | --- | --- | --- |
| `TG-001/task_01` | Domain contract | backend | high | One-line deliverable and boundary | None | `UT-001`, `IT-002` |
| `TG-001/task_02` | Runtime integration | backend | medium | One-line deliverable and boundary | `TG-001/task_01` | `IT-003`, `E2E-001` |

Use qualified task IDs in approval output so repeated local `task_01` names do
not collapse across task groups. List exact test IDs, not only counts. When
`_tests.md` is absent, list the task's concrete inline cases in `Assigned tests`
instead. After any task group or task edit, recompute dependencies, ownership, and
exactly-once test assignment, then render the entire proposal again. Ask for
approval only when every task group has at least one task row and the
initiative-wide test totals reconcile with `_tests.md`.

Cancellation is no-write outcome:

- First-time cancellation leaves no marker, `_task_groups/`, task group task, or
  temporary artifact.
- Existing-plan cancellation preserves last confirmed bytes, checksum, and
  task group directories.
- Read-only planning returns `task_group_plan_read_only` with bytes unchanged.
- Confirmation from old checksum fails as stale write and never overwrites newer
  confirmed plan.
- Repeating confirmation reopens existing plan once; it does not duplicate nodes
  or directories.

Large proposals remain complete. Render all task group candidates, task rows, IDs,
rationales, exact test assignments, and diagnostics; use keyboard navigation,
search where available, and screen-reader labels containing ID, title, outcome,
completion, and unmet dependency count. Never truncate collection or hide a
blocker.

## Confirmed artifact contract

Only opt-in marker is initiative-root
`.compozy/tasks/<initiative>/_task_groups.md`. Stable public identity remains
`<initiative>/TG-NNN`. New plans use one-level readable physical directories;
`NNN` must match the stable ID and `<brief>` is a lowercase kebab-case summary
of the approved task group title:

```text
.compozy/tasks/<initiative>/_task_groups.md
.compozy/tasks/<initiative>/_task_groups/001-shared-foundation/_tasks.md
.compozy/tasks/<initiative>/_task_groups/001-shared-foundation/task_01.md
```

Persist the declared directory in `_task_groups.md` and resolve it from that
manifest; never reconstruct it from `TG-NNN`. Existing `_task_groups/TG-NNN/`
directories remain valid and stay byte-for-byte stable during plan edits. A
persisted readable directory also remains stable when its display title changes
unless the user explicitly approves a directory migration.

Root plan must use this exact `compozy.task-groups/v1` hybrid YAML/Markdown
shape. The root field is `initiative`; `workflow` belongs only in each Task
Group-local `_tasks.md` manifest.

```markdown
---
schema_version: compozy.task-groups/v1
initiative: customer-management
graph:
  nodes:
    - id: TG-001
      directory: _task_groups/001-shared-foundation
    - id: TG-002
      directory: _task_groups/002-api-delivery
  edges:
    - from: TG-001
      to: TG-002
      rationale: Shared contracts must exist before API delivery
---

# customer-management Task Groups

## [ ] TG-001 — Shared foundation

- Reference: `customer-management/TG-001`
- Outcome: Shared contracts and persistence are ready
- Owns:
  - Domain contracts
  - Persistence primitives
- Dependencies: None

## [ ] TG-002 — API delivery

- Reference: `customer-management/TG-002`
- Outcome: Customer management API is ready
- Owns:
  - HTTP handlers
  - API integration tests
- Dependencies:
  - `TG-001` — Shared contracts must exist before API delivery
```

Every YAML node has exactly one body heading in the literal form
`## [ ] TG-NNN — Title` or `## [x] TG-NNN — Title`. Completion is stored only
in that checkbox; do not emit `- Completed:`. Each body contains `Reference`,
`Outcome`, a non-empty indented `Owns` list, and `Dependencies`. Use
`- Dependencies: None` when no edge enters the Task Group. Otherwise list every
incoming dependency as ``  - `TG-NNN` — rationale`` with text exactly matching
the corresponding YAML edge rationale. Free-form headings such as `## Summary`
do not represent Task Groups.

The plan must also preserve stable IDs, exact directories, hierarchical
references, non-empty title/outcome/scope, acyclic graph edges, mirrored
dependency lists, and heading checkbox state.

Every task group task manifest uses `workflow: <initiative>/TG-NNN`. Each task is
owned by containing task group and qualified for initiative-wide audits as
`<task-group-id>/<task-id>`; `TG-001/task_01` and `TG-002/task_01` are distinct,
while one qualified task in two manifests is duplicate-owner error.

Generation is one validated publication boundary. Stage root plan, every
task group `_tasks.md`, and every task file in memory/temporary storage; validate
paths, manifests, metadata, ownership, and test assignments before publishing.
On any write, sync, close, rename, permission, or validation error, report
failure and leave last confirmed executable state intact. Do not copy `_prd.md`,
`_techspec.md`, `_user_stories.md`, `_tests.md`, or ADRs into task group.

Before success, audit all `_tests.md` IDs across whole initiative exactly once,
including tests assigned to different task group suites. Report orphan, duplicate,
or cross-task-group ownership diagnostics with qualified task group IDs. Audit remains
initiative-wide even when one task group is displayed or generated separately.

## Verification scenarios

Run these as skill-level contract checks; full definitions live in
`.compozy/tasks/nested-workflows/_tests.md`.

| ID | Required observable result |
| --- | --- |
| UT-001 | Inseparable PRD/TechSpec map returns `ordinary`, non-empty rationale, no task group proposal. |
| UT-002 | Proposal with 250 valid candidates renders all 250 stable IDs and rationales without truncation. |
| UT-004 | Zero-node proposal returns `ordinary_recommended` and does not render executable marker. |
| IT-001 | Missing PRD reports canonical path and creates neither `_task_groups.md` nor `_task_groups/`. |
| IT-002 | Missing TechSpec blocks task group readiness and preserves existing ordinary task files. |
| IT-003 | Existing valid plan opens in edit mode with no write before confirmation. |
| IT-004 | Cancelled first proposal leaves no marker, task group directory, task file, or temporary file. |
| IT-005 | Read-only writer returns `task_group_plan_read_only` and leaves planning artifacts byte-identical. |
| IT-006 | Stale session confirmation fails without overwriting newer confirmed bytes. |
| IT-007 | Reconfirming persisted proposal edits it once without duplicate nodes/directories. |
| IT-008 | Cancelling existing-plan edits preserves last confirmed checksum and task group directories. |
| IT-009 | Confirmed generation writes readable `NNN-brief` directories, a valid root plan, and one valid manifest per task group; permits qualified repeated task names and assigns every test ID exactly once. |
| IT-010 | Concurrent plan mutation yields one checksum-consistent validation or concurrent-change error, never mixed graph/body result. |
| IT-039 | Concurrent ownership edit accepts one writer; stale submission fails and next prompt reads confirmed root plan. |
| E2E-001 | The neutral task table and total count appear before a four-strategy menu with concrete task/group counts, one recommendation, and reasons for unavailable strategies. Smaller ordinary tasks render a revised table before approval; either Task Group strategy then presents every task group's complete task table and revised count before approval. Confirmed Task Groups yield `_task_groups.md` plus readable task-group-local suites; confirmed ordinary delivery yields root task files with no marker. |
| E2E-002 | Split/combine/rename/reorder/dependency edits persist exactly as confirmed and reopen with same graph/content. |
| E2E-003 | A 100-task-group proposal is keyboard/screen-reader navigable across first/middle/last items; cancellation causes no data loss or truncation. |

Successful planning stops after validation. It never creates branches, runs
tasks, invokes review, checks completion, or starts another task group.
