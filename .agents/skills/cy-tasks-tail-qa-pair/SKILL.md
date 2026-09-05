---
name: cy-tasks-tail-qa-pair
description: "Append missing QA planning/execution tasks to a spec package for a requested cy-loop-tasks run, including UI E2E coverage where applicable. Excludes ordinary task lists, ideation, and review-round tasks."
trigger: explicit
---

# Tasks Tail QA Pair

Append the canonical QA pair (`$qa-report` + `$qa-execution`) to the `_tasks.md` of a spec-cycle package that will run under `cy-loop-tasks`, so the loop closes the program with a real verification pass. Ordinary fixes and routine task lists outside a requested full loop do not get the pair; `references/qa-tail-template.md` and `cy-spec-preflight` `tasks-checks.md` carry the same rule. The pair operates on the repo's living QA tree (`docs/qa/`) — plans become journeys/charters/scenario files, results become registry bugs and dated reports. The tail complements per-slice verification, never replaces it: each slice ships with its own `## Shippable Outcome` evidence, and the tail walks cross-slice journeys plus anything that changed after a slice's evidence was recorded — it does not re-walk untouched per-slice results.

## Procedures

**Step 1: Locate the Tasks File**

1. Resolve the target `_tasks.md` path. If invoked immediately after `cy-create-tasks`, the orchestrator passes the slug; otherwise read the most recently modified `.compozy/tasks/<slug>/_tasks.md`.
2. Read the file and check whether the last two non-empty entries already follow the QA pair pattern.
3. If both `qa-report` and `qa-execution` rows exist with proper dependencies, exit with status `noop` — do not duplicate.

**Step 2: Detect UI-Bearing Features**

1. If the slug directory contains `_uiux.md`, set `requires_e2e=true` — the spec marked the feature UI-bearing.
2. Otherwise parse the task list for any task that touches `web/`, `packages/site`, `web/e2e/`, Storybook, or any frontend-facing surface; if at least one does, set `requires_e2e=true`.
3. If no task touches UI but the spec covers public API/CLI, agent-manageability, extensibility, or config lifecycle surfaces, set `requires_cli_e2e=true`.
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
