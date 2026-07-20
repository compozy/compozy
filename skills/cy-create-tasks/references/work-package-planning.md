# Work Package Planning Contract

Use this reference when maintainer chooses Work Package planning. It is opt-in
planning layer over one canonical PRD and TechSpec, not delivery orchestrator.

## Preconditions and recommendation

1. Enter this contract only after `cy-create-tasks` has rendered the complete
   neutral task table and total task count. The user must know the proposed
   tasks, ownership, dependencies, and exact test assignments before choosing a
   delivery shape.
2. Read and parse `.compozy/tasks/<initiative>/_prd.md` and
   `.compozy/tasks/<initiative>/_techspec.md` completely. Verify both readable
   and canonical before offering Work Packages.
3. Offer `ordinary` (one general workflow) and `work_packages` after the neutral
   breakdown. Ordinary preserves the existing task directory, `_tasks.md`, task
   names, commands, and behavior; creates no `_work_packages.md` or `_packages/`.
   Declining Work Packages needs no reason.
4. Recommend `ordinary` when the displayed implementation map has zero nodes or one
   inseparable component. Render non-empty rationale and
   `ordinary_recommended`; never render executable zero-node proposal.
5. Recommend Work Packages only for coherent, reviewable outcomes. Explain for
   each candidate stable ID, title, outcome, owned scope, dependencies,
   dependency rationale, and independent peers. File/task/line counts warn;
   they do not enforce boundaries.

## Editable proposal state

Keep proposal state in current session until confirmation. State includes source
checksum, stable `WP-NNN` IDs, physical directory, title, outcome, owned scope,
dependency edges and rationales, independent groups, current checkbox state,
package-local task graphs, and exact test assignments.

Existing valid `_work_packages.md` is an edit session: load it, preserve IDs,
physical directories, and `[ ]`/`[x]` states, and show current checksum. Do not
silently replace it or create duplicate package directories. Proposal may add,
remove, split, combine, rename, reorder, edit package or task edges, and revise
task ownership/test assignments. Removing a referenced package shows every
affected dependent edge before edit can be confirmed. Blank title/outcome,
unknown/self dependencies, cycles, duplicate IDs, incomplete task tables, and
ownership conflicts block confirmation.

## Approval presentation contract

The neutral task preview is the first decision surface. Selecting Work Packages
opens this second approval surface; it does not approve package boundaries or
write artifacts. If package design merges or splits neutral tasks, list every
change and the revised total task count before asking for confirmation.

Approval covers both package boundaries and the tasks inside them. Render every
package in dependency order with its directory, outcome, owned scope, package
dependencies and rationales, independent peers, and test totals. Immediately
under each package, render the complete proposed task table:

| Task | Title | Type | Complexity | Scope | Depends on | Assigned tests |
| --- | --- | --- | --- | --- | --- | --- |
| `WP-001/task_01` | Domain contract | backend | high | One-line deliverable and boundary | None | `UT-001`, `IT-002` |
| `WP-001/task_02` | Runtime integration | backend | medium | One-line deliverable and boundary | `WP-001/task_01` | `IT-003`, `E2E-001` |

Use qualified task IDs in approval output so repeated local `task_01` names do
not collapse across packages. List exact test IDs, not only counts. When
`_tests.md` is absent, list the task's concrete inline cases in `Assigned tests`
instead. After any package or task edit, recompute dependencies, ownership, and
exactly-once test assignment, then render the entire proposal again. Ask for
approval only when every package has at least one task row and the
initiative-wide test totals reconcile with `_tests.md`.

Cancellation is no-write outcome:

- First-time cancellation leaves no marker, `_packages/`, package task, or
  temporary artifact.
- Existing-plan cancellation preserves last confirmed bytes, checksum, and
  package directories.
- Read-only planning returns `work_package_plan_read_only` with bytes unchanged.
- Confirmation from old checksum fails as stale write and never overwrites newer
  confirmed plan.
- Repeating confirmation reopens existing plan once; it does not duplicate nodes
  or directories.

Large proposals remain complete. Render all package candidates, task rows, IDs,
rationales, exact test assignments, and diagnostics; use keyboard navigation,
search where available, and screen-reader labels containing ID, title, outcome,
completion, and unmet dependency count. Never truncate collection or hide a
blocker.

## Confirmed artifact contract

Only opt-in marker is initiative-root
`.compozy/tasks/<initiative>/_work_packages.md`. Stable public identity remains
`<initiative>/WP-NNN`. New plans use one-level readable physical directories;
`NNN` must match the stable ID and `<brief>` is a lowercase kebab-case summary
of the approved package title:

```text
.compozy/tasks/<initiative>/_work_packages.md
.compozy/tasks/<initiative>/_packages/001-shared-foundation/_tasks.md
.compozy/tasks/<initiative>/_packages/001-shared-foundation/task_01.md
```

Persist the declared directory in `_work_packages.md` and resolve it from that
manifest; never reconstruct it from `WP-NNN`. Existing `_packages/WP-NNN/`
directories remain valid and stay byte-for-byte stable during plan edits. A
persisted readable directory also remains stable when its display title changes
unless the user explicitly approves a directory migration.

Root plan must satisfy `compozy.work-packages/v1` manifest and readable Markdown
body contract from `_techspec.md`: stable IDs, exact directories, hierarchical
references, non-empty title/outcome/scope, dependency rationales, acyclic graph,
mirrored dependency lists, and heading checkbox state.

Every package task manifest uses `workflow: <initiative>/WP-NNN`. Each task is
owned by containing package and qualified for initiative-wide audits as
`<package-id>/<task-id>`; `WP-001/task_01` and `WP-002/task_01` are distinct,
while one qualified task in two manifests is duplicate-owner error.

Generation is one validated publication boundary. Stage root plan, every
package `_tasks.md`, and every task file in memory/temporary storage; validate
paths, manifests, metadata, ownership, and test assignments before publishing.
On any write, sync, close, rename, permission, or validation error, report
failure and leave last confirmed executable state intact. Do not copy `_prd.md`,
`_techspec.md`, `_user_stories.md`, `_tests.md`, or ADRs into package.

Before success, audit all `_tests.md` IDs across whole initiative exactly once,
including tests assigned to different package suites. Report orphan, duplicate,
or cross-package ownership diagnostics with qualified package IDs. Audit remains
initiative-wide even when one package is displayed or generated separately.

## Verification scenarios

Run these as skill-level contract checks; full definitions live in
`.compozy/tasks/nested-workflows/_tests.md`.

| ID | Required observable result |
| --- | --- |
| UT-001 | Inseparable PRD/TechSpec map returns `ordinary`, non-empty rationale, no package proposal. |
| UT-002 | Proposal with 250 valid candidates renders all 250 stable IDs and rationales without truncation. |
| UT-004 | Zero-node proposal returns `ordinary_recommended` and does not render executable marker. |
| IT-001 | Missing PRD reports canonical path and creates neither `_work_packages.md` nor `_packages/`. |
| IT-002 | Missing TechSpec blocks package readiness and preserves existing ordinary task files. |
| IT-003 | Existing valid plan opens in edit mode with no write before confirmation. |
| IT-004 | Cancelled first proposal leaves no marker, package directory, task file, or temporary file. |
| IT-005 | Read-only writer returns `work_package_plan_read_only` and leaves planning artifacts byte-identical. |
| IT-006 | Stale session confirmation fails without overwriting newer confirmed bytes. |
| IT-007 | Reconfirming persisted proposal edits it once without duplicate nodes/directories. |
| IT-008 | Cancelling existing-plan edits preserves last confirmed checksum and package directories. |
| IT-009 | Confirmed generation writes readable `NNN-brief` directories, a valid root plan, and one valid manifest per package; permits qualified repeated task names and assigns every test ID exactly once. |
| IT-010 | Concurrent plan mutation yields one checksum-consistent validation or concurrent-change error, never mixed graph/body result. |
| IT-039 | Concurrent ownership edit accepts one writer; stale submission fails and next prompt reads confirmed root plan. |
| E2E-001 | The neutral task table and total count appear before the `ordinary`/`work_packages` choice. Choosing Work Packages then presents every package's complete task table and revised count before approval, yielding `_work_packages.md` plus readable package-local suites; choosing ordinary yields general workflow locations with no marker. |
| E2E-002 | Split/combine/rename/reorder/dependency edits persist exactly as confirmed and reopen with same graph/content. |
| E2E-003 | A 100-package proposal is keyboard/screen-reader navigable across first/middle/last items; cancellation causes no data loss or truncation. |

Successful planning stops after validation. It never creates branches, runs
tasks, invokes review, checks completion, or starts another package.
