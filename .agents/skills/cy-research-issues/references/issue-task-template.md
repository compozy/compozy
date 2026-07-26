# Issue-Task File Template & Suite Contract

## Contents

- §Template — the full `task_NN.md` file template (issue DNA + executable contract)
- §Parser Rules — naming, frontmatter, and status contract
- §Complexity Criteria — how to assign the `complexity` field
- §Graph Manifest — the `_tasks.md` `compozy.tasks/v2` contract
- §Test-Plan Rules — Unit/Integration/E2E requirements and suite placement
- §General Rules — independence, severity, cross-surface duties

## Template

Every generated task file merges **issue DNA** (problem, evidence, expected behavior) with the
**executable task contract** consumed by `cy-execute-task`.

```markdown
---
status: pending
title: [Imperative title — what the fix/change accomplishes]
type: [one of the allowed type slugs from .compozy/config.toml [tasks].types or the defaults]
complexity: [low | medium | high | critical]
---

# Task NN: [Title — must match frontmatter title]

## Overview
[2-3 sentences: the user-visible symptom or gap first, the technical cause second, and why fixing
it matters. This is the issue's "Problem" statement.]

<critical>
- ALWAYS READ `_techspec.md` and `analysis/summary.md` before starting
- REFERENCE the TechSpec section named below for the target design — do not re-derive it
- FOCUS ON "WHAT" — subtasks describe outcomes, not implementation steps
- MINIMIZE CODE in this file — evidence snippets only
- TESTS REQUIRED — the Tests section below is the executor's test checklist; tests exist to find
  bugs, never weaken assertions to pass
</critical>

<requirements>
- [Numbered MUST/SHOULD requirements — specific and verifiable, derived from Expected Behavior]
- [e.g., "MUST render a retryable error pane when the transcript fetch fails"]
- [e.g., "MUST NOT open a popover; params/outputs expand inline under the row"]
</requirements>

## Problem & Evidence
[The defect/gap in detail. Every claim cites `path:line`.]

- `path/to/file.ts:123` — [what this line proves]
- `internal/pkg/file.go:45-67` — [what this range proves]
- Source analysis: `analysis/NN_analysis_<slug>.md §section` [deep dive lives there]

## Expected Behavior
[The target behavior/design. When copying a competitor pattern, cite the exact reference:]

- Copy target: `.resources/<repo>/path/to/Component.tsx:100-150` — [which pattern]
- Deliberate divergence: [where and why the project deviates from the competitor]

## Subtasks
- [ ] NN.1 [Outcome-focused subtask]
- [ ] NN.2 [...]
- [ ] NN.3 [Verification subtask when the task changes visible UI: capture screenshots]

## Implementation Details
[Integration points and constraints. Reference `_techspec.md §<section>` for the design.
List delete targets explicitly when replacing code — zero-legacy, no compat shims.]

### Relevant Files
- `path/to/file` — [why it changes]

### Dependent Files
- `path/to/consumer` — [why it is affected]

### Competitor References
- `.resources/<repo>/path` — [what to copy/adapt from it]

## Deliverables
- [Concrete output 1 — code/behavior]
- [Delete target removed: `path/to/dead-code`]
- Unit tests per the Tests section **(REQUIRED)**
- Integration tests per the Tests section **(REQUIRED)**

## Tests
- Unit tests (suite: `path/to/existing.test.ts[x]` or `path/pkg/*_test.go`):
  - [ ] [Specific input/condition → expected output, e.g., "transcript fetch 500 → error pane with retry, stream still opens"]
  - [ ] [Edge/boundary case]
- Integration tests (suite: `<path or make target>`):
  - [ ] [Cross-component flow, e.g., "reconnect with stale Last-Event-ID on idle active session → non-empty render inputs"]
- E2E tests (lane: `make test-e2e-runtime` / `make test-e2e-web`, or Not applicable — <reason>):
  - [ ] [User-visible flow, e.g., "open → navigate away → return on a running session shows the transcript"]
- Test coverage target: >=80% for touched packages
- All tests must pass under the repo gates (`-race` for Go)

## Success Criteria
- All tests passing; coverage >=80% on touched packages
- [Measurable outcome, e.g., "8 consecutive identical tool calls render as one grouped cluster"]
- [For UI tasks: screenshot captured via the repo's UI-screenshot skill and cited]
- [Observability/telemetry hooks landed when the parent techspec requires them]
```

## Parser Rules

- File name matches `task_\d+\.md` with zero-padded numbers (`task_01.md` … `task_99.md`). The
  `task_` prefix has no leading underscore; underscore-prefixed names are reserved for meta
  documents (`_prd.md`, `_techspec.md`, `_tasks.md`).
- The file MUST start with YAML frontmatter containing **only** `status`, `title`, `type`,
  `complexity`. Never put `dependencies` in task frontmatter.
- `title` must match the first H1 in the body.
- Valid `status` values: `pending`, `in_progress`, `completed` (`done`/`finished` treated as
  completed). Generated tasks start `pending`.
- `type` comes from `.compozy/config.toml` `[tasks].types` when configured; otherwise the
  defaults: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.

## Complexity Criteria

- `low` — single file change, no new interfaces, no concurrency, straightforward logic.
- `medium` — 2-4 files, may introduce a new interface or struct, limited integration points.
- `high` — 5+ files, new subsystem or significant refactor, multiple integration points, or
  concurrency involved.
- `critical` — cross-cutting change affecting many packages, high regression risk, requires
  coordination with other tasks.

## Graph Manifest (`_tasks.md`)

`_tasks.md` is the canonical (and only) home of dependency relationships. It MUST start with:

```markdown
---
schema_version: "compozy.tasks/v2"
workflow: <slug>
graph:
  nodes:
    - id: task_01
      file: task_01.md
  edges:
    - from: task_01
      to: task_02
---

# [Title] Task List
```

- `workflow` MUST match the task directory name.
- `graph.nodes` includes every task exactly once; node `id` is the canonical `task_NN` and `file`
  matches it (`task_NN.md`).
- Each edge means `from` must finish before `to` starts. Use `edges: []` when independent. Edges
  encode **true dependencies only** (task B needs task A's output) — the graph MUST be acyclic.
- Wave/priority grouping is advisory: record it as a "Suggested execution order" section in the
  `_tasks.md` body, never as fake edges.

## Test-Plan Rules

- Every category (Unit, Integration, E2E) is present. A category may read
  `Not applicable — <reason>`, never be silently omitted.
- Each test case names the specific input, condition, or expected behavior — "test the happy
  path" is a defect.
- Name the canonical owning suite for each case; extend existing suites (per the repo's
  test-placement rules / `consolidate-test-suites`) and create a new file only when no suite owns
  the invariant — then name the new file explicitly.
- Never generate a task dedicated solely to testing another task's code; tests ride inside the
  task that changes the behavior. (A cross-cutting test-infrastructure task is allowed.)

## General Rules

- Independently implementable: the task must be completable when its `_tasks.md` edges are met —
  no undeclared coupling. Tight coupling → merge the tasks or extract a shared dependency task.
- The `## Problem & Evidence` and `## Expected Behavior` sections make these files reviewable as
  *issues*; the remaining sections make them *executable*. Both are mandatory.
- Cross-surface work (backend contract changes) must name the codegen co-ship duty (OpenAPI + TS
  types + E2E mocks) inside Implementation Details.
- Severity is expressed through `_tasks.md` ordering + the techspec waves, not a frontmatter field
  (the parser does not know "severity"). Mention severity in Overview prose when useful.
- Minimize code in task bodies; reference the TechSpec for patterns instead of duplicating them.
