# Analysis: production-surfaces

## Scope

- Slice question: how the Tasks, Jobs, and Triggers production surfaces render loading, error, unfiltered-empty, and filtered-empty states; which shared UI primitives and domain composites own those regions; and which canonical tests, stories, and routes should be changed without duplicating invariants.
- Primary sources: `web/src/systems/tasks/**`, `web/src/systems/automation/**`, `web/src/systems/os/apps/tasks/**`, `web/src/systems/os/apps/automation/**`, `packages/ui/src/index.ts`, `packages/ui/src/components/empty.tsx`, `COPY.md`, `DESIGN.md`, and `web/CLAUDE.md`.
- Sources read in full: the production composites, route/page view models, filter helpers, route stories, component stories, canonical component/page tests, `Empty`, the UI barrel, and the named product/design/frontend instruction files cited below.
- Sources sampled: the 388 candidate files under the four scoped systems were surveyed by filename/search; only the empty-state and route/filter ownership files were selected for deep reading.
- Adjacent-slice overlap: this covers production state ownership and verification seams, not visual parity or implementation of the redesign itself.

## Overview

The Tasks route has two different empty-state owners. `useTasksPage` treats an unfiltered zero-total list as the special route empty state, while a query/status/owner/priority-filtered zero result falls through to the list or Kanban composite. `TasksCatalogLocation` checks scope, dashboard, and inbox branches before the `page.isEmpty` branch, so dashboard and inbox have their own state contracts; only list/Kanban share the route-level “no tasks yet” gate (`web/src/systems/tasks/hooks/use-tasks-page.ts:65-80,200-210`; `web/src/systems/os/apps/tasks/tasks-catalog-location.tsx:126-239`).

Jobs and Triggers already have a well-factored shared catalog state envelope. The page hooks own URL search/filter state, runtime-unavailable errors, pagination, and create/clear callbacks; the thin Jobs/Triggers catalog composites pass those values to `AutomationCatalogShell`. The shell distinguishes loading, empty-with-error, empty-with-filters, empty-without-filters, populated rows/cards, pagination errors, and load-more (`web/src/systems/os/apps/automation/use-automation-page-base.ts:100-196`; `web/src/systems/automation/components/automation-catalog-shell.tsx:28-157`).

The main production gaps for a filtered-empty redesign are therefore asymmetric: Tasks list already has a filtered message but no clear-filters action; Tasks Kanban renders five per-column “No tasks” messages without a board-level filtered state; Tasks Inbox only asks whether any loaded item exists and cannot distinguish no inbox work from filters hiding all work. Jobs and Triggers have the correct branching shape, but their tests and full-route stories do not yet assert both unfiltered and filtered empty actions.

## Mechanisms / Patterns

- **Tasks route-level zero-state gate:** `useTasksPage.isEmpty` requires an active scope, no loading/error, a zero server total, and no list filters (`use-tasks-page.ts:200-210`). `TasksCatalogLocation` renders `TasksEmptyState` for that case before selecting Kanban or List (`tasks-catalog-location.tsx:202-239`). This prevents a generic list empty from replacing the task-template onboarding surface.
- **Tasks onboarding composite:** `TasksEmptyState` owns the “No tasks yet” headline, optional workspace name, New task CTA, optional CLI CTA, and four template cards (`tasks-empty-state.tsx:39-113`). It deliberately sets `fill={false}` because its scrollable template grid is not a single full-height `Empty` surface (`tasks-empty-state.tsx:57-100`). The route currently passes only `onSelectTemplate` and `workspaceName`, so the `onCopyCli` story path is not wired into production (`tasks-catalog-location.tsx:56-60,202-204`).
- **Tasks list filtered branch:** `TasksListSurface` computes `visibleCount` from the server-projected task tree and `hasFilters` from `filterState` or search text (`tasks-list-surface.tsx:44-50`). Loading wins, then zero-row error, then zero-row empty; the filtered branch uses Search and “No tasks match the current filters,” while the unfiltered branch uses ListChecks and “No tasks yet” (`tasks-list-surface.tsx:55-97`). The composite receives no clear-filters callback, so its filtered copy has no recovery control.
- **Tasks Kanban per-column fallback:** `TasksKanbanBoard` owns board loading, zero-row errors, columns, partial facet counts, pagination, and retry (`tasks-kanban-board.tsx:47-155`). `TaskKanbanColumn` decides emptiness from its children and defaults every empty column to “No tasks” (`task-kanban-column.tsx:49-106`). This is useful column-level context, but a filtered result with no cards currently looks identical to an unfiltered board with no cards unless the route-level `TasksEmptyState` gate applies.
- **Tasks Inbox item-based empty detection:** `useTasksInboxView` flattens `inbox.groups`, partitions items into display groups, builds filter chips, and returns only `hasItems: allItems.length > 0` (`use-tasks-inbox-view.ts:37-42,83-109`). `TasksInboxView` always renders its search/filter/unread toolbar, then chooses loading, error, one generic empty, or grouped rows (`tasks-inbox-view.tsx:117-206`). It does not receive a `hasActiveFilters`/clear callback, so a filtered zero result is announced as “Nothing is waiting in the inbox.”
- **Tasks dashboard state contract:** `TasksDashboardView` owns dashboard loading, no-data, and error states, then composes scheduler controls and dashboard sections (`tasks-dashboard-view.tsx:39-119`). Dashboard has no list filter semantics, so its existing `No dashboard data yet` state should remain a separate aggregate-data state.
- **Shared automation filter ownership:** `useAutomationPageBase` serializes search, scope, source, enabled, event, view, and clear operations into route search; `hasActiveFilters` includes all non-default values (`use-automation-page-base.ts:137-195`). `AutomationListFilters` projects the typed state into the shipped `FiltersWithSearch` primitive (`automation-list-filters.tsx:25-78`).
- **Shared Jobs/Triggers shell:** `AutomationCatalogShell` chooses skeletons when loading with no items, an error `Empty` when no items and an error exists, and a filtered or unfiltered `Empty` otherwise. Filtered state gets a “Clear filters” action; unfiltered state gets “Create job” or “Create trigger” (`automation-catalog-shell.tsx:49-110`). Populated rows/cards retain later-page errors and explicit load-more controls (`automation-catalog-shell.tsx:114-157`).
- **Primitive reuse and fill semantics:** `@compozy/ui` exposes the primitive inventory through `packages/ui/src/index.ts:1-11`; the `Empty` primitive supports icon/title/description/cause/action, `framed`, and `fill`, with `fill` defaulting to true unless framed (`packages/ui/src/components/empty.tsx:11-27,41-75`). Existing automation tests correctly assert the flex envelope and `data-fill="true"` (`automation-catalog.test.tsx:158-165`).

## Relevant Sources

- `web/src/systems/tasks/hooks/use-tasks-page.ts:124-171,200-210` — server-owned task filters, derived list/Kanban data, and the unfiltered-only `isEmpty` predicate.
- `web/src/systems/os/apps/tasks/tasks-catalog-location.tsx:87-124,126-239` — topbar/filter composition and production branch order for scope, dashboard, inbox, onboarding empty, Kanban, and list.
- `web/src/systems/tasks/components/tasks-empty-state.tsx:39-113` — task onboarding empty state, template grid, CTAs, and fill choice.
- `web/src/systems/tasks/components/tasks-list-surface.tsx:19-30,44-150` — list loading/error/empty/populated/pagination state machine and filtered copy.
- `web/src/systems/tasks/components/tasks-inbox-view.tsx:25-56,90-239` — inbox props, toolbar, generic empty branch, error, and continuation controls.
- `web/src/systems/tasks/hooks/use-tasks-inbox-view.ts:37-109` — inbox flattening/grouping and current `hasItems` implementation.
- `web/src/systems/tasks/components/tasks-kanban-board.tsx:32-155` — board error, columns, loading skeletons, facet totals, retry, and pagination.
- `web/src/systems/tasks/components/task-kanban-column.tsx:49-106` — column-level “No tasks” fallback and create control.
- `web/src/systems/tasks/components/tasks-dashboard-view.tsx:39-119` — dashboard loading, error, no-data, and aggregate rendering.
- `web/src/systems/automation/components/automation-catalog-shell.tsx:15-193` — shared Jobs/Triggers state envelope and filtered/unfiltered CTA branching.
- `web/src/systems/automation/components/automation-jobs-catalog.tsx:11-74` and `web/src/systems/automation/components/automation-triggers-catalog.tsx:11-56` — domain wrappers that keep shell ownership shared.
- `web/src/systems/os/apps/automation/use-automation-page-base.ts:100-196` — URL-backed filter state, clear operation, and `hasActiveFilters`.
- `web/src/systems/os/apps/automation/use-automation-jobs-page.ts:20-85` and `web/src/systems/os/apps/automation/use-automation-triggers-page.ts:13-63` — page view models supplying rows, errors, loading, pagination, and empty actions.
- `web/src/systems/automation/components/__tests__/automation-catalog.test.tsx:110-172` — canonical shell test suite; currently covers geometry, pagination, and loading but not filtered/unfiltered CTA copy/action.
- `web/src/systems/tasks/components/__tests__/tasks-list-surface.test.tsx:155-176` — canonical list empty, filtered-empty, loading, and error assertions.
- `web/src/systems/tasks/components/__tests__/tasks-inbox-view.test.tsx:163-172` — canonical inbox loading/error/empty assertion, currently without a filtered-empty distinction.
- `web/src/systems/tasks/components/__tests__/tasks-kanban-board.test.tsx:40-90,205-226` — canonical board/column empty and loading/error coverage; no board-level filtered-empty invariant.
- `web/src/systems/tasks/routes/tasks.stories.tsx:39-59,61-105,107-151` — real-shell Tasks route stories for empty, mode changes, loading, and dashboard error.
- `web/src/systems/automation/routes/jobs.stories.tsx:36-111` and `web/src/systems/automation/routes/triggers.stories.tsx:36-116` — real-shell Jobs/Triggers route stories for cards, unfiltered empty, errors, editor, and loading; neither has a filtered-empty route story.
- `web/src/systems/tasks/components/stories/tasks-list-surface.stories.tsx:174-197`, `tasks-kanban-board.stories.tsx:113-147`, and `tasks-inbox-view.stories.tsx:205-241` — component stories for current unfiltered empty/loading/error states; add filtered variants in these existing stories if component-level visual coverage is needed.
- `packages/ui/src/components/empty.tsx:11-120` and `packages/ui/src/index.ts:1-11` — shipped `Empty` contract and public primitive barrel.
- `COPY.md:444-457,556-568` — UI empty states must state truth, explain why data is absent, and provide the next useful action; canonical “No <object> yet” pattern.
- `DESIGN.md:971-982,994-1013` — plain state copy, actionable empty/error contracts, and reuse of the shared `<Empty>` primitive.
- `web/CLAUDE.md:101-110` — page-level orchestration, server-owned totals/facets, and the requirement to handle loading/error/empty states.

## Transferable Patterns

- **Keep state precedence in the production composite:** extend `TasksListSurface`, `TasksKanbanBoard`, and `TasksInboxView` with the smallest state inputs they need, while leaving API/query ownership in `useTasksPage` and `useTasksInboxView`. This follows the page/route orchestration rule and prevents each story from inventing a second state machine.
- **Use one canonical filtered-empty invariant per surface:** retain the existing filtered list assertion in `tasks-list-surface.test.tsx`; add the missing clear-action assertion there if the list gets a callback. Add one board-level filtered-empty assertion to `tasks-kanban-board.test.tsx` and one inbox filtered-empty assertion to `tasks-inbox-view.test.tsx`. Do not duplicate these same invariants in hook tests unless the hook itself changes its public contract.
- **Make route stories verify the real branch:** add filtered-empty MSW stories to `routes/tasks.stories.tsx`, `routes/jobs.stories.tsx`, and `routes/triggers.stories.tsx` because these stories mount the real shell and route/page view models. Keep component stories for local layout variants, not as a substitute for route wiring.
- **Reuse the existing automation shell pattern for Tasks only where semantics match:** Jobs/Triggers already pass `hasActiveFilters`, `onClearFilters`, and `onCreate`; a Tasks list/Kanban or Inbox clear action should use the route’s existing search setters/clear operation rather than a local reset that leaves URL state stale.
- **Preserve domain-specific unfiltered onboarding:** the route-level `TasksEmptyState` template grid and Jobs/Triggers create CTAs are not interchangeable. Use the shared `Empty` primitive for the state head, but keep domain composites responsible for their next action and supporting controls.
- **Use `Empty` fill deliberately:** full listing envelopes should keep the default fill behavior; custom onboarding grids should opt out as `TasksEmptyState` does. If a redesign introduces framed cards, use the primitive’s `framed` contract rather than hand-building a generic empty card.
- **Cover the four states explicitly:** every route story set should make loading, error, unfiltered empty, and filtered empty observable. Existing route stories already cover the first three for most surfaces; the missing filtered branches are the high-value additions.

## Risks / Mismatches

- **Changing only `TasksListSurface` will miss the production zero state.** With no filters and `total === 0`, `TasksCatalogLocation` short-circuits to `TasksEmptyState`; list/Kanban receive the empty tree only when filters are active or data exists (`use-tasks-page.ts:203-210`; `tasks-catalog-location.tsx:202-239`).
- **A generic “No tasks” board message hides filter intent.** The board currently always renders canonical columns, and each empty column says “No tasks” (`tasks-kanban-board.tsx:80-122`; `task-kanban-column.tsx:97-105`). A board-level filtered state must not remove useful columns unless the approved design explicitly calls for it; otherwise keep column context and add one clear, truthful recovery surface.
- **Inbox item count is not filter truth.** `hasItems` is derived from flattened loaded groups, while the toolbar can change lane, status, priority, unread, and search state (`use-tasks-inbox-view.ts:92-109`; `tasks-inbox-view.tsx:117-187`). Do not infer an unfiltered empty from `allItems.length` alone; pass or derive an explicit filter-active signal and preserve server totals/facets.
- **Do not treat a loaded page as a complete catalog.** The frontend rules require server-owned scope/filter/count/page data (`web/CLAUDE.md:103-106`). Empty copy and counts must be based on the query envelope/facets, not on a client-trimmed list or a guessed total.
- **Error precedence must remain intact.** Automation gives a zero-row error priority over empty, but preserves rows with a later-page error (`automation-catalog-shell.tsx:53-67,132-140`). Tasks list/Kanban/Inbox have the same distinction; a filtered-empty redesign must not replace a known daemon error with “no matches.”
- **Current task onboarding copy may over-explain mechanics.** `TasksEmptyState` puts a long runtime description below its heading (`tasks-empty-state.tsx:89-92`), while the design/copy rules prefer plain state, next action, and no decorative explanatory text (`COPY.md:444-457`; `DESIGN.md:971-982`). Any copy change should be justified by the approved redesign and runtime truth, not by adding another subtitle.
- **The CLI CTA is demonstrated but not wired.** `TasksEmptyState` supports `onCopyCli`, and its component story covers it, but `TasksCatalogLocation` does not pass that handler (`tasks-empty-state.tsx:39-43,75-86`; `tasks-catalog-location.tsx:202-204`). Do not expose a button in the production state unless the route owns a real copy/action path.
- **Tests can drift into implementation checks.** Existing tests use stable test IDs and user-visible text for state contracts; new tests should assert the state and action, not Tailwind classes or the exact internal arrangement of `Empty` children except where fill/layout is already the primitive contract.

## Open Questions

- Should filtered Tasks List, Kanban, and Inbox states all expose a single “Clear filters” action, or should one surface retain a non-actionable message? The current list has no clear callback, Kanban has no filter prop, and Inbox has no active-filter signal.
- For filtered Kanban, should the five status columns remain visible with a board-level empty message, or should the empty message replace the grid? This is a product/design decision; the current component contract supports column-level empties only.
- Should the production Tasks onboarding empty state expose the optional `compozy tasks new` copy CTA? The component supports it, but the route has no handler in the scoped production path.
- Should route-level filtered-empty stories assert clearing the URL query and reloading rows, or only assert the visible action? The former needs the route’s canonical MSW response sequence and should not be duplicated in component tests.
- Does the redesign change dashboard or inbox copy, or only list/catalog empties? Dashboard is an aggregate no-data state and currently has no filters; changing it would be a separate invariant from catalog filtered-empty behavior.

## Evidence

- `web/src/systems/tasks/hooks/use-tasks-page.ts`
- `web/src/systems/os/apps/tasks/tasks-catalog-location.tsx`
- `web/src/systems/tasks/components/tasks-empty-state.tsx`
- `web/src/systems/tasks/components/tasks-list-surface.tsx`
- `web/src/systems/tasks/components/tasks-inbox-view.tsx`
- `web/src/systems/tasks/hooks/use-tasks-inbox-view.ts`
- `web/src/systems/tasks/components/tasks-kanban-board.tsx`
- `web/src/systems/tasks/components/task-kanban-column.tsx`
- `web/src/systems/tasks/components/tasks-dashboard-view.tsx`
- `web/src/systems/automation/components/automation-catalog-shell.tsx`
- `web/src/systems/automation/components/automation-jobs-catalog.tsx`
- `web/src/systems/automation/components/automation-triggers-catalog.tsx`
- `web/src/systems/automation/components/automation-list-filters.tsx`
- `web/src/systems/os/apps/automation/use-automation-page-base.ts`
- `web/src/systems/os/apps/automation/use-automation-jobs-page.ts`
- `web/src/systems/os/apps/automation/use-automation-triggers-page.ts`
- `web/src/systems/automation/components/__tests__/automation-catalog.test.tsx`
- `web/src/systems/tasks/components/__tests__/tasks-list-surface.test.tsx`
- `web/src/systems/tasks/components/__tests__/tasks-inbox-view.test.tsx`
- `web/src/systems/tasks/components/__tests__/tasks-kanban-board.test.tsx`
- `web/src/systems/tasks/routes/tasks.stories.tsx`
- `web/src/systems/automation/routes/jobs.stories.tsx`
- `web/src/systems/automation/routes/triggers.stories.tsx`
- `web/src/systems/tasks/components/stories/tasks-list-surface.stories.tsx`
- `web/src/systems/tasks/components/stories/tasks-kanban-board.stories.tsx`
- `web/src/systems/tasks/components/stories/tasks-inbox-view.stories.tsx`
- `packages/ui/src/components/empty.tsx`
- `packages/ui/src/index.ts`
- `COPY.md`
- `DESIGN.md`
- `web/CLAUDE.md`
