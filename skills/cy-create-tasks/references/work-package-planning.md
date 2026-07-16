# Work Package Planning Contract

Use this reference when maintainer chooses Work Package planning. It is opt-in
planning layer over one canonical PRD and TechSpec, not delivery orchestrator.

## Preconditions and recommendation

1. Read and parse `.compozy/tasks/<initiative>/_prd.md` and
   `.compozy/tasks/<initiative>/_techspec.md` completely. Verify both readable
   and canonical before presenting Work Package choice.
2. Offer `ordinary` and `work_packages`. Ordinary preserves existing task
   directory, `_tasks.md`, task names, commands, and behavior; creates no
   `_work_packages.md` or `_packages/`. Declining Work Packages needs no reason.
3. Recommend `ordinary` when implementation map has zero nodes or one
   inseparable component. Render non-empty rationale and
   `ordinary_recommended`; never render executable zero-node proposal.
4. Recommend Work Packages only for coherent, reviewable outcomes. Explain for
   each candidate stable ID, title, outcome, owned scope, dependencies,
   dependency rationale, and independent peers. File/task/line counts warn;
   they do not enforce boundaries.

## Editable proposal state

Keep proposal state in current session until confirmation. State includes source
checksum, stable `WP-NNN` IDs, title, outcome, owned scope, dependency edges and
rationales, independent groups, and current checkbox state.

Existing valid `_work_packages.md` is an edit session: load it, preserve IDs and
`[ ]`/`[x]` states, and show current checksum. Do not silently replace it or
create duplicate package directories. Proposal may add, remove, split, combine,
rename, reorder, or edit edges/rationales. Removing referenced package shows
every affected dependent edge before edit can be confirmed. Blank title/outcome,
unknown/self dependencies, cycles, duplicate IDs, and ownership conflicts block
confirmation.

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

Large proposals remain complete. Render all candidates, IDs, rationales, and
diagnostics; use keyboard navigation, search where available, and screen-reader
labels containing ID, title, outcome, completion, and unmet dependency count.
Never truncate collection or hide blocker.

## Confirmed artifact contract

Only opt-in marker is initiative-root
`.compozy/tasks/<initiative>/_work_packages.md`. It references one-level hidden
storage:

```text
.compozy/tasks/<initiative>/_work_packages.md
.compozy/tasks/<initiative>/_packages/WP-001/_tasks.md
.compozy/tasks/<initiative>/_packages/WP-001/task_01.md
```

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
| IT-009 | Confirmed generation writes valid root plan and one valid manifest per package, permits qualified repeated task names, and assigns every test ID exactly once. |
| IT-010 | Concurrent plan mutation yields one checksum-consistent validation or concurrent-change error, never mixed graph/body result. |
| IT-039 | Concurrent ownership edit accepts one writer; stale submission fails and next prompt reads confirmed root plan. |
| E2E-001 | Choosing Work Packages yields `_work_packages.md` plus package-local suites; choosing ordinary yields legacy locations with no marker. |
| E2E-002 | Split/combine/rename/reorder/dependency edits persist exactly as confirmed and reopen with same graph/content. |
| E2E-003 | A 100-package proposal is keyboard/screen-reader navigable across first/middle/last items; cancellation causes no data loss or truncation. |

Successful planning stops after validation. It never creates branches, runs
tasks, invokes review, checks completion, or starts another package.
