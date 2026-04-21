# Task Memory: task_11.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

Build the read-oriented workflow task board (`/workflows/$slug/tasks`) and task detail (`/workflows/$slug/tasks/$taskId`) surfaces against the additive daemon `GET /api/tasks/{slug}/board` and `GET /api/tasks/{slug}/items/{task_id}` contracts, reusing the task_09/task_10 shell + typed client patterns, without introducing any browser authoring affordances (ADR-004).

## Important Decisions

- Tasks subsystem lives inside the existing `@/systems/workflows` barrel (adapters/hooks/components/types) instead of a new `tasks` system — board and detail are workflow-scoped reads and share view-model concerns.
- Routes use the flat-sibling escape (`workflows_.$slug.tasks.tsx`, `workflows_.$slug.tasks_.$taskId.tsx`) so they do not try to nest under the leaf `_app/workflows.tsx` that has no `<Outlet />`, matching the `runs_.$runId.tsx` precedent.
- Broadened `toTransportError` in `web/src/lib/api-client.ts` to walk `.cause` so operator-visible error alerts surface the inner transport message even when it is wrapped in an `ApiRequestError`; this also quietly upgrades dashboard/runs error UX.
- Workflow navigation is delivered as explicit "Open task board" affordances on every `workflow-row-*` card plus a linked workflow title, instead of adding a new top-level shell nav entry — task surfaces are workflow-specific and chain cleanly from inventory.

## Learnings

- Running `bun scripts/tsr-generate.mjs` (not `bun run scripts/...`) directly regenerates `routeTree.gen.ts` when iterating on flat-sibling routes; the `_` suffix on a segment in the filename writes a route that escapes the matching leaf layout.
- `installFetchStub` returns JSON bodies with a fixed `content-type: application/json` header — it does not need special handling for 404 TransportError bodies, but `matchUrl(path, "GET")` still needs to include the method or POSTs will bleed into GET fixtures.
- `useWorkflowBoard`/`useWorkflowTask` must guard null workspace/slug/taskId inside the queryFn instead of only via `enabled`, otherwise TanStack Query's default initial render triggers the fn before React resolves the route params.

## Files / Surfaces

- `web/src/systems/workflows/adapters/tasks-api.ts` + `tasks-api.test.ts`
- `web/src/systems/workflows/hooks/use-tasks.ts`
- `web/src/systems/workflows/lib/query-keys.ts` (extended)
- `web/src/systems/workflows/components/task-board-view.tsx` + `task-board-view.test.tsx`
- `web/src/systems/workflows/components/task-detail-view.tsx` + `task-detail-view.test.tsx`
- `web/src/systems/workflows/components/workflow-inventory-view.tsx` (+ router context in its test) to link into the board
- `web/src/systems/workflows/index.ts`, `types.ts` (barrel + generated OpenAPI aliases)
- `web/src/routes/_app/workflows_.$slug.tasks.tsx`
- `web/src/routes/_app/workflows_.$slug.tasks_.$taskId.tsx`
- `web/src/routes/-workflow-tasks.integration.test.tsx`
- `web/src/lib/api-client.ts` (`toTransportError` cause-unwrap)

## Errors / Corrections

- Initial component tests for `WorkflowInventoryView` broke when the row title became a `<Link>` — wrapped the component render in a TanStack Router `createRouter` shell so `useLinkProps` has a router context (see `workflow-inventory-view.test.tsx`).
- Integration tests originally asserted on the raw TransportError message but `apiErrorMessage` falls back to the caller-provided string when the error is wrapped as `ApiRequestError`; fixed by enhancing `toTransportError` to walk `.cause`.

## Ready for Next Run

- `web/src/systems/workflows/index.ts` now exports `TaskBoardView`, `TaskDetailView`, `useWorkflowBoard`, `useWorkflowTask`, `getWorkflowBoard`, `getWorkflowTask`, and the typed `TaskBoardPayload`/`TaskDetailPayload` aliases for downstream Storybook (task_13) and QA (task_16) slices.
- Board card link: `workflow-view-board-<slug>` / task link: `task-board-link-<task_id>` / run link on detail: `task-detail-run-link-<run_id>` — stable testIds for later E2E/QA.
