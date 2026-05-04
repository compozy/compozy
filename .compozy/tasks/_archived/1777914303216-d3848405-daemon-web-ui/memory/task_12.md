# Task Memory: task_12.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Build the v1 reviews, spec, and memory operator surfaces on the already-wired daemon contract: reviews list/detail with review-fix dispatch, workflow spec read (PRD/TechSpec/ADRs), and workflow memory read using opaque `file_id`s.

## Important Decisions
- `/reviews` index reuses `useDashboard` for workflow summaries then fans out per-workflow `listReviewIssues` via TanStack `useQueries` rather than adding a new cross-workflow aggregate endpoint — keeps scope tight to the existing v1 daemon contract.
- Memory initial selection picks the `shared` entry (or first entry) so the detail route always hydrates the document panel before the user clicks. Selection is stored in route-local state keyed by opaque `file_id`.
- Review detail route validates the `$round` path param locally (integer ≥ 0) before hitting the daemon — invalid rounds render an inline transport-style alert instead of firing a failing query.
- Routes use the flat-sibling escape (`reviews_.$slug.$round.$issueId.tsx`, `workflows_.$slug.spec.tsx`, `memory_.$slug.tsx`) so the detail pages do not inherit the listing layout.

## Learnings
- `matchUrl(pattern)` helpers using `url.includes(...)` can silently match more specific nested paths (e.g. `/api/tasks/alpha/memory` also matches `/api/tasks/alpha/memory/files/file-shared`). When a route fetches both an index and a child document, use `matchPath(...)` (endsWith) in the fetch stubs to avoid the nested matcher swallowing the child request.
- `useQueries` in TanStack Query v5 returns an array aligned with the input order — safe to index by position when building per-card view models.
- `openapi-fetch` passes typed header params through `params.header`, which is already the canonical way to forward `X-Compozy-Workspace-ID` from adapters in this repo.

## Files / Surfaces
- `web/src/systems/reviews/**` — types, adapters, hooks, list/detail views, barrel.
- `web/src/systems/spec/**` — types, adapter, hook, workflow spec view, barrel.
- `web/src/systems/memory/**` — types, adapters, hooks, memory index + workflow memory views, barrel.
- `web/src/routes/_app/reviews.tsx` — index route composing dashboard + per-card issues.
- `web/src/routes/_app/reviews_.$slug.$round.$issueId.tsx` — issue detail + dispatch action.
- `web/src/routes/_app/workflows_.$slug.spec.tsx` — PRD/TechSpec/ADR tabs.
- `web/src/routes/_app/memory.tsx` + `web/src/routes/_app/memory_.$slug.tsx` — memory index + detail.
- `web/src/systems/app-shell/components/app-shell-layout.tsx` — Reviews + Memory entries added to "Across workflows" nav.
- `web/src/systems/workflows/components/workflow-inventory-view.tsx` — Spec/Memory deep links per active workflow row.
- `web/src/routes/-reviews-flow.integration.test.tsx`, `web/src/routes/-spec-memory-flow.integration.test.tsx` — route-level integration coverage.

## Errors / Corrections
- None blocking. Initial memory detail test run failed because `matchUrl("/api/tasks/alpha/memory")` also matched the file endpoint; swapped to `matchPath(...)` (endsWith) and the recover-on-file-404 assertion now sees the inner transport message.

## Ready for Next Run
- Storybook/MSW work (task_13) can depend on the `@/systems/reviews`, `@/systems/spec`, `@/systems/memory` barrels and the route test-id vocabulary introduced here (`reviews-index-*`, `review-detail-*`, `workflow-spec-*`, `memory-index-*`, `workflow-memory-*`).
