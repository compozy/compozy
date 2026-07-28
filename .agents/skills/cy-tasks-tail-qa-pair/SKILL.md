---
name: cy-tasks-tail-qa-pair
description: Appends a qa-report planning task and a qa-execution task at the end of every cy-create-tasks output, wired to the living docs/qa contract (scenario files, journeys, charters, bug registry, dated reports). Adds e2e coverage (Playwright or browser-use) for UI-bearing features. Use after cy-create-tasks finishes generating _tasks.md and the file lacks the trailing QA pair. Do not use for tasks generated outside the Compozy PRD pipeline, for ideation/brainstorming output, or for review-round task lists.
trigger: explicit
---

# Tasks Tail QA Pair

Auto-append the canonical QA pair (`$qa-report` + `$qa-execution`) to every `_tasks.md` produced by `cy-create-tasks`, so the implementation agent always closes a program with a real verification pass. The pair operates on the repo's living QA tree (`docs/qa/`) — plans become journeys/charters/scenario files, results become registry bugs and dated reports.

## Procedures

**Step 1: Locate the Tasks File**

1. Resolve the target `_tasks.md` path. If invoked immediately after `cy-create-tasks`, the orchestrator passes the slug; otherwise read the most recently modified `.compozy/tasks/<slug>/_tasks.md`.
2. Read the file and check whether the last two non-empty entries already follow the QA pair pattern.
3. If both `qa-report` and `qa-execution` rows exist with proper dependencies, exit with status `noop` — do not duplicate.

**Step 2: Detect UI-Bearing Features**

1. Parse the task list for any task that touches `web/`, `packages/site`, `web/e2e/`, Storybook, or any frontend-facing surface.
2. If at least one task touches user-facing UI, set `requires_e2e=true`.
3. If no task touches UI but the techspec covers public API/CLI, agent-manageability, extensibility, or config lifecycle surfaces, set `requires_cli_e2e=true`.
4. Otherwise `requires_e2e=false` (rare — backend-only refactors).

**Step 3: Read the Tail Template**

1. Read `references/qa-tail-template.md` for the canonical row shape, complexity rating, and required `<critical>` blocks.
2. Note the `Dependencies` syntax: the `qa-report` task depends on the last implementation task, and the `qa-execution` task depends on `qa-report`.
3. Preserve the table column order used in the existing `_tasks.md` (do not reorder columns). The current canonical order is `# | Title | Status | Complexity | Dependencies`.

**Step 4: Compose the QA Pair**

1. Generate the `qa-report` task row using the template:
   - Title: `QA Plan and Session Charters`
   - Frontmatter type: `qa-report`
   - Status: `pending`
   - Complexity: `high`
   - Dependencies: last implementation task ID
2. Generate the `qa-execution` task row:
   - Title: `Real-User QA Execution`
   - Frontmatter type: `qa-execution`
   - Status: `pending`
   - Complexity: `critical`
   - Dependencies: the new `qa-report` task ID
   - Include e2e directive when `requires_e2e=true` (Playwright via `browser-use:browser`, fallback to `agent-browser`).
   - Include CLI/API/agent-manageability end-to-end directive when `requires_cli_e2e=true`.
3. Compute correct sequential task IDs (e.g., next `task_NN` numbers).

**Step 5: Append and Verify**

1. Append the two rows below the existing list. Do not modify earlier rows.
2. Create matching `task_NN.md` files for both QA rows using the body guidance in `references/qa-tail-template.md`.
3. If the `_tasks.md` includes a `## MVP Boundary` section that references "tasks 01-NN", update only the QA range to include the new tasks.
4. Read `references/qa-pair-checklist.md` and confirm every item passes before exit.
5. Print the final two-row diff to stdout for human/agent review.

## Error Handling

- If the target `_tasks.md` cannot be located, fail loudly and report the resolved slug. Do not write files speculatively.
- If the file lacks a recognizable task table (e.g., it is empty or uses a custom format), refuse to edit; emit the discovered shape on stderr and ask for the correct path.
- If `cy-create-tasks` already inserted partial QA tasks (only `qa-report` or only `qa-execution`), repair the missing half rather than duplicating.
- Never replace existing QA rows. If the user has customized them, treat the file as ready and exit `noop`.
