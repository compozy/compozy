# Technical Specification: Daemon Web UI Runtime App for Compozy

## Executive Summary

This specification introduces the official daemon web UI for Compozy as a daemon-served React SPA, using the same runtime frontend structure and toolchain used in `/Users/pedronauck/dev/compozy/agh`, while implementing the product model and visual direction defined by `docs/design/daemon-mockup`. No `_prd.md` exists for this feature. This document is derived from the approved clarification sequence, the accepted daemon architecture under `.compozy/tasks/daemon/`, direct exploration of the current daemon code, the local mockup, and the AGH runtime frontend structure.

The implementation keeps the daemon as the only control plane. The browser reads and mutates state only through daemon-owned REST and SSE contracts, with OpenAPI as the canonical web contract and generated types in the frontend. Production remains single-binary and local-first: the daemon serves the SPA at `/`, preserves the API at `/api`, and embeds `web/dist` with `go:embed`. The primary trade-off is deliberate cross-stack work in Go, frontend workspace tooling, and API contract generation in exchange for a coherent operator console that matches the mockup, starts with the daemon by default, and avoids split-brain state between browser, CLI, and workspace files.

The web UI is additive to the current daemon API, not a silent contract replacement. User-facing browser terminology follows the mockup and uses “workflow” in the shell. Transport compatibility remains aligned with the current `/api/tasks`, `/api/reviews`, `/api/runs`, `/api/sync`, and `/api/workspaces` surfaces unless a later daemon API version explicitly introduces a breaking resource rename.

## System Architecture

### Component Overview

1. `Frontend workspace`
   - Add a Bun workspace rooted in the repository with `web/` and `packages/ui/`.
   - `web/` hosts the daemon SPA using React 19, TanStack Router, TanStack Query, Vite, Vitest, Storybook, Playwright, MSW, Tailwind CSS v4, shadcn, Zustand, Zod, `openapi-fetch`, and the AGH-style route/system structure.
   - `packages/ui/` hosts shared UI primitives, theme tokens, typography, and shell composition helpers.
   - The package structure mirrors AGH's runtime frontend slices, not its broader monorepo.

2. `Theme and visual system`
   - Reuse AGH's structural frontend architecture, not AGH's product visuals.
   - Tokens and typography are derived from `docs/design/daemon-mockup/colors_and_type.css`.
   - The default theme is dark-first with Compozy daemon branding, lime signal accents, mono metadata labels, and self-hosted display/mono fonts from the mockup assets, subject to font-license verification and defined fallbacks.

3. `Embedded asset serving`
   - Add `web/embed.go` that embeds `web/dist`.
   - Extend `internal/api/httpapi` to serve exact static assets at `/`, bypass `/api`, and fall back to `index.html` for SPA routes using the AGH `static.go` model.
   - Keep UDS API-only. Browser UI is served only from the daemon HTTP listener on `127.0.0.1`.

4. `Daemon web contract layer`
   - Extend `internal/api/core` with typed DTOs and handlers for dashboard, workflow overview, spec, memory, task board/detail, review detail, and richer run surfaces.
   - Preserve the current root-scoped API family and add browser read models as siblings under existing resources.
   - Publish a checked-in OpenAPI document for the daemon HTTP API and generate typed frontend bindings in the same style used by AGH.

5. `Daemon read-model services`
   - Reuse `global.db` for workflow, task, review, and run summary projections.
   - Reuse `run.db` and existing run manager streaming for run detail and live event views.
   - Serve PRD, TechSpec, ADR, and memory content through daemon-side document readers that read canonical workspace files on the server. The browser never touches the filesystem directly.
   - Use read-through document caching with mtime-based invalidation plus explicit invalidation from existing daemon sync/watch events for the active workflow.

6. `Operational action surfaces`
   - Support daemon-backed mutations required by the v1 operator console: sync, archive, run start, run cancel, and review-fix dispatch.
   - Keep PRD, TechSpec, task, and memory authoring outside the browser in v1.

7. `Browser security boundary`
   - Treat the localhost HTTP listener as lower-trust than UDS because it is browser reachable.
   - Extend daemon HTTP middleware for strict Host/Origin validation, no wildcard CORS, and CSRF protection on mutating endpoints.

### Route Model

The SPA uses TanStack Router file-based layout conventions, mirroring AGH’s runtime app structure:

- `web/src/routes/__root.tsx` for root error/not-found boundaries
- `web/src/routes/_app.tsx` for the global app shell
- `web/src/routes/_app/index.tsx` for dashboard
- `web/src/routes/_app/workflows.tsx`
- `web/src/routes/_app/workflows.$slug.spec.tsx`
- `web/src/routes/_app/workflows.$slug.tasks.tsx`
- `web/src/routes/_app/workflows.$slug.tasks.$taskId.tsx`
- `web/src/routes/_app/runs.tsx`
- `web/src/routes/_app/runs.$runId.tsx`
- `web/src/routes/_app/reviews.tsx`
- `web/src/routes/_app/reviews.$slug.$round.$issueId.tsx`
- `web/src/routes/_app/memory.tsx`
- `web/src/routes/_app/memory.$slug.tsx`

The browser route tree uses “workflow” as the shell noun because that matches the mockup. The daemon transport contract keeps the current task/review route families for compatibility.

### Active Workspace Model

The SPA is single-workspace-per-tab by design in v1. This is a deliberate constraint, not an inference.

- On bootstrap, the shell calls `GET /api/workspaces`.
- If zero workspaces exist, the shell renders an empty-state picker and offers `POST /api/workspaces/resolve`.
- If one workspace exists, the shell auto-selects it.
- If many workspaces exist, the shell requires explicit selection.
- The active workspace ID is the daemon registry UUID returned by the workspace transport layer.
- The selected workspace ID is persisted in `sessionStorage`, not daemon server session state, so tabs do not silently fight each other.
- Requests that need workspace context send `X-Compozy-Workspace-ID`.
- If the workspace ID is stale or deleted, the daemon returns a typed problem response and the shell drops back to workspace selection.
- Switching workspace closes active SSE subscriptions, invalidates query caches, and reloads the shell against the new workspace.

### Streaming Contract

The web UI reuses the existing `/api/runs/:run_id/stream` surface and makes its semantics explicit:

- Media type: `text/event-stream`
- Event types:
  - `run.snapshot`
  - `run.event`
  - `run.heartbeat`
  - `run.overflow`
- Each emitted event carries a stable monotonic cursor in the SSE `id` field.
- The client reconnects using `Last-Event-ID`; the server may also accept an explicit `cursor` query parameter for non-browser clients.
- Heartbeat cadence is 15 seconds unless the daemon transport contract changes globally.
- If the replay boundary is unavailable, the server emits `run.overflow` and the client must refetch `GET /api/runs/:run_id/snapshot` before reopening the stream.
- Ordering is per-run and append-only from the daemon event bus / run DB writer path.
- OpenAPI remains the canonical request/response contract for REST. SSE semantics are documented in this TechSpec and the route-level API docs because OpenAPI does not model SSE well enough on its own.

### Data Flow

1. The browser requests `/` from the daemon HTTP listener.
2. `internal/api/httpapi` serves the embedded SPA bundle.
3. The root shell resolves the active workspace from daemon-backed workspace endpoints and tab-local session state.
4. Route loaders and query hooks call the OpenAPI-generated client against `/api`, attaching `X-Compozy-Workspace-ID` where needed.
5. Dashboard, workflow inventory, review lists, and run summaries read daemon projections from `global.db`.
6. Run detail and live status use daemon snapshot endpoints and SSE streams backed by `run.db` and the existing run manager.
7. Spec and memory pages fetch daemon document payloads; the daemon reads canonical markdown files server-side and serves normalized typed responses.
8. Operational actions post back to the daemon, then invalidate query state and rehydrate the affected views.
9. Storybook route stories use MSW to mock the same `/api` contracts for isolated review and regression coverage.

## Implementation Design

### Core Interfaces

```go
type WebDocumentService interface {
	WorkflowSpec(ctx context.Context, workspaceID string, workflowSlug string) (WorkflowSpecDocument, error)
	WorkflowMemoryIndex(ctx context.Context, workspaceID string, workflowSlug string) (WorkflowMemoryIndex, error)
	WorkflowMemoryFile(ctx context.Context, workspaceID string, workflowSlug string, fileID string) (MarkdownDocument, error)
	TaskDetail(ctx context.Context, workspaceID string, workflowSlug string, taskID string) (TaskDetailPayload, error)
}
```

```go
type ActiveWorkspaceResolver interface {
	Resolve(ctx context.Context, path string) (Workspace, error)
	Get(ctx context.Context, id string) (Workspace, error)
	List(ctx context.Context) ([]Workspace, error)
}
```

```go
type EmbeddedWebBundle interface {
	FS() (fs.FS, error)
}
```

Error handling follows existing daemon conventions:
- service methods return wrapped errors using `%w`
- handler code translates domain failures into the shared problem envelope
- mutating endpoints reject missing or invalid `X-Compozy-Workspace-ID` with explicit transport errors

### Data Models

#### Repository and Package Topology

- root `package.json`, `tsconfig.json`, workspace scripts, and Bun lockfile
- `web/` for the SPA
- `packages/ui/` for reusable UI primitives and tokens
- checked-in `openapi/compozy-daemon.json`
- no `packages/site` in this feature

#### Frontend Module Structure

- `web/src/routes/` for file-based route shells
- `web/src/components/` for app shell composition
- `web/src/systems/<domain>/` for domain modules
- `web/src/lib/` for shared runtime helpers and the generated API client
- `web/src/generated/` for generated OpenAPI typings
- `web/src/storybook/` and `web/src/routes/**/stories/` for Storybook and route-state fixtures

Primary domain systems:

- `dashboard`
- `workflows`
- `runs`
- `reviews`
- `spec`
- `memory`
- `app-shell`

#### Backend Read Models

- `WorkspaceDashboard`
  - daemon status, health summary, active runs, queue summary, pending reviews, activity feed
- `WorkflowCard`
  - slug, title, status, phase, provider, branch, timestamps, task counts, review counts
- `TaskBoardPayload`
  - workflow summary plus kanban lanes and task cards
- `TaskDetailPayload`
  - task metadata, latest task status, task memory summary, related run summary, live tail availability
- `RunDetailPayload`
  - run snapshot, provider/model/runtime config, job counters, token usage, timeline, live stream cursor
- `ReviewDetailPayload`
  - workflow slug, round, issue metadata, source location, body, proposed patch, discussion summary
- `WorkflowSpecDocument`
  - PRD, TechSpec, ADR list and markdown bodies
- `WorkflowMemoryIndex`
  - memory entries with opaque `file_id`, relative display path, kind, size, and updated time
- `MarkdownDocument`
  - title, updated at, raw markdown, rendered metadata

#### Source of Truth Rules

- `global.db`
  - workflow summaries, task/review/run indexes, queue state, registry state
- `run.db`
  - run events, transcript, live stream replay
- workspace filesystem
  - canonical PRD, TechSpec, ADR, and memory markdown documents
- browser
  - no source of truth; query cache only

#### Document Read and Cache Strategy

- The daemon is authoritative for document reads.
- For spec and memory endpoints, the daemon uses read-through caching keyed by workspace ID, workflow slug, document kind, and file mtime.
- When active workflow watchers or sync events report relevant file changes, the document cache is invalidated immediately.
- When no watcher is active, the daemon revalidates mtime before serving cached content.
- Memory file fetches use daemon-issued opaque `file_id` values from the index endpoint, not raw relative paths.
- The daemon rejects path traversal by resolving `file_id` through the indexed memory file table, never by trusting a browser-supplied path segment.

### API Endpoints

The browser extends the current daemon API surface additively. Existing root-scoped resources remain canonical.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/daemon/status` | Daemon identity, version, uptime, listener info, workspace/run counts |
| `GET` | `/api/daemon/health` | Ready/degraded state for dashboard health strip |
| `GET` | `/api/daemon/metrics` | Existing metrics exposition, extended with web metrics |
| `GET` | `/api/workspaces` | Registered workspaces for active workspace selection |
| `POST` | `/api/workspaces/resolve` | Resolve or register a workspace by path |
| `GET` | `/api/ui/dashboard` | Dashboard aggregate payload for the active workspace |
| `GET` | `/api/tasks` | Existing workflow inventory for the active workspace |
| `GET` | `/api/tasks/:slug` | Existing workflow overview payload |
| `GET` | `/api/tasks/:slug/spec` | PRD, TechSpec, and ADR read payloads |
| `GET` | `/api/tasks/:slug/memory` | Memory file index for one workflow |
| `GET` | `/api/tasks/:slug/memory/files/:file_id` | One memory notebook body |
| `GET` | `/api/tasks/:slug/board` | Kanban/list task payload |
| `GET` | `/api/tasks/:slug/items/:task_id` | Task detail payload including related run state |
| `POST` | `/api/tasks/:slug/runs` | Existing workflow run start from the active workspace |
| `POST` | `/api/tasks/:slug/archive` | Existing archive surface |
| `POST` | `/api/sync` | Existing workflow sync surface scoped by active workspace |
| `GET` | `/api/reviews/:slug` | Existing latest review summary |
| `GET` | `/api/reviews/:slug/rounds/:round/issues` | Existing issue list |
| `GET` | `/api/reviews/:slug/rounds/:round/issues/:issue_id` | Review issue detail payload |
| `POST` | `/api/reviews/:slug/rounds/:round/runs` | Existing review-fix run start |
| `GET` | `/api/runs` | Cross-workflow run inventory |
| `GET` | `/api/runs/:run_id` | Stable run summary |
| `GET` | `/api/runs/:run_id/snapshot` | Rich run detail snapshot |
| `GET` | `/api/runs/:run_id/stream` | SSE stream for live run updates |
| `POST` | `/api/runs/:run_id/cancel` | Cancel a run |

Status conventions:
- `200` for synchronous reads
- `202` when a mutation starts a long-running daemon action
- `404` when workspace, workflow, task, review, memory file, or run is missing
- `409` for invalid concurrent operations such as duplicate run starts
- `412` for missing or stale active workspace context
- shared problem envelope for all non-success JSON responses

### OpenAPI Generation Contract

OpenAPI is the canonical browser contract and is checked in, mirroring AGH:

- source: `openapi/compozy-daemon.json`
- generator: `openapi-typescript`
- generated types output: `web/src/generated/compozy-openapi.d.ts`
- typed transport client: `web/src/lib/api-client.ts` using `openapi-fetch`
- root commands:
  - `bun run codegen`
  - `bun run codegen-check`
- `codegen-check` fails if regenerating types produces a diff

The spec does not require Go server stubs from OpenAPI. The daemon remains the implementation source; the checked-in OpenAPI document is the browser-facing contract artifact.

## Integration Points

No new runtime external services are introduced. The feature integrates with:
- the existing local daemon HTTP API
- the existing daemon run manager and databases
- the new local frontend toolchain and code generation workflow

### Security

The browser-facing listener requires explicit defenses:

- accept only loopback Host authorities that match the daemon bind host and active HTTP port
- reject unexpected `Origin` headers on all browser endpoints
- do not enable wildcard CORS
- require same-origin CSRF protection for mutating browser endpoints using a double-submit token pattern
- SSE stays same-origin and uses the same Host/Origin validation path
- UDS remains the higher-trust CLI transport and does not share browser cookies or CSRF state

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
| --- | --- | --- | --- |
| root JS workspace config | new | Adds Bun workspace, scripts, TS config, and frontend build flow. Medium risk because repo currently has no JS workspace. | Add workspace files, install contract, and verification commands. |
| `web/` | new | Adds the daemon SPA and route/domain architecture. Medium risk because it introduces a second implementation stack. | Create shell, routes, systems, and generated client integration. |
| `packages/ui/` | new | Adds shared UI primitives and tokens. Low to medium risk if boundaries are not enforced. | Mirror AGH runtime UI package structure and mockup theme tokens. |
| `openapi/` | new | Adds the canonical browser contract artifact. Medium risk if it drifts from handlers. | Add checked-in JSON, codegen, and codegen-check. |
| `internal/api/httpapi` | modified | Gains embedded asset serving, SPA fallback routing, and browser hardening. Medium risk because bad routing can shadow `/api`. | Add static FS loading, Host/Origin checks, bypass rules, and tests. |
| `internal/api/core` | modified | Gains richer web-facing DTOs, handlers, and CSRF / workspace middleware. Medium risk because current contract is CLI-first. | Add additive read-model endpoints and explicit workspace context handling. |
| `internal/daemon/*transport_service.go` | modified | Must compose new read models from DB plus workspace documents. Medium risk because read models span multiple sources. | Add workflow/spec/memory/review/task query services and doc-cache invalidation. |
| build and verify pipeline | modified | Must build web assets before embedding and enforce frontend verification. Medium risk because verify is currently Go-only. | Extend Makefile and bootstrapping docs with Bun-aware gates. |

## Testing Approach

### Unit Tests

- Vitest for `packages/ui` primitives, shell behavior, route loaders, view-model hooks, and domain formatters
- generated client contract tests for success, empty payload, and problem-envelope handling
- Go unit tests for static asset resolution, SPA fallback bypass rules, Host/Origin validation, CSRF validation, and embedded bundle loading
- MSW-backed route stories and isolated component tests for loading, empty, success, degraded, and error states

### Integration Tests

- Go integration tests for daemon HTTP serving embedded assets while preserving `/api`
- Playwright against the daemon serving embedded assets, not only Vite dev mode
- seeded daemon integration flows for:
  - dashboard load
  - workflow inventory and drill-down
  - run detail and live stream reconnect
  - review detail and review-fix dispatch
  - spec and memory read views
  - workflow sync, archive, run start, and run cancel
- Storybook coverage for route-level and component-level mocked states using MSW

## Development Sequencing

### Build Order

1. Add the Bun workspace, root frontend tooling, install contract, and `web/` plus `packages/ui/` package skeletons. Depends on no previous steps.
2. Add `openapi/compozy-daemon.json`, `codegen`, and `codegen-check`, and wire generated types into `web/`. Depends on step 1.
3. Implement `packages/ui` tokens, typography, shadcn primitives, and mockup-aligned shell components. Depends on step 1.
4. Implement daemon read-model services and additive REST endpoints for dashboard, spec, memory, task detail, and review detail. Depends on step 2.
5. Add active-workspace middleware, document-cache invalidation, SSE contract handling, and browser security middleware. Depends on step 4.
6. Add `web/embed.go`, daemon HTTP static serving, SPA fallback, and asset-serving tests. Depends on steps 1 and 5.
7. Build the SPA route tree, domain systems, query hooks, and operational actions against the typed client. Depends on steps 2, 3, 4, and 5.
8. Add Storybook route stories and MSW handlers for all major UI states. Depends on steps 3 and 7.
9. Extend the repository verification contract with Bun-aware gates and embedded-asset bootstrap rules. Depends on steps 1, 2, and 6.
10. Add Playwright daemon-served E2E coverage and final verification wiring. Depends on steps 5, 6, 7, 8, and 9.

### Technical Dependencies

- Bun must be part of the local/CI tool-install contract
- `web/dist/` must contain a checked-in placeholder file so fresh-checkout `go build` does not fail before the first frontend build
- `make build` must execute frontend build before Go build
- `make verify` must run:
  - Bun install/bootstrap check
  - web lint
  - web typecheck
  - web test
  - web build
  - go fmt
  - go lint
  - go test
  - go build

## Monitoring and Observability

Operational visibility remains local-first and daemon-owned. New web metrics are emitted through the existing `GET /api/daemon/metrics` text exposition.

Key metrics:
- `daemon_http_web_requests_total`
- `daemon_http_web_asset_miss_total`
- `daemon_http_web_spa_fallback_total`
- `daemon_web_api_errors_total`
- `daemon_web_run_stream_reconnect_total`
- `daemon_web_action_latency_seconds`

The daemon metrics response remains Prometheus 0.0.4 text output. New metric labels must stay low-cardinality and reuse existing daemon request labeling where possible.

## Technical Considerations

### Key Decisions

- Decision: mirror only AGH's runtime frontend slices, not the broader monorepo.
- Decision: serve the SPA from the daemon's existing HTTP listener and embed assets into the binary.
- Decision: keep the browser daemon-only and contract-driven with OpenAPI-generated types.
- Decision: keep the current root-scoped API families and add browser read models additively rather than silently replacing the daemon contract with `/workspaces/:id/workflows/...`.
- Decision: make the SPA single-workspace-per-tab in v1 using sessionStorage plus `X-Compozy-Workspace-ID`.
- Decision: require explicit SSE contract semantics and browser-listener security rules in the spec itself.
- Decision: require full frontend verification including Storybook and MSW.

### Known Risks

- The current daemon API is CLI-first, so new read models may sprawl unless handler boundaries stay tight.
- Server-side document reads can become inconsistent if watcher-driven invalidation is not implemented carefully.
- Frontend workspace adoption can slow verification until Bun/bootstrap behavior is standardized in the repo.
- Browser security on localhost is easy to underestimate; Host/Origin and CSRF enforcement must not be deferred.

## Architecture Decision Records

- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md)
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md)
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md)
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md)
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md)
