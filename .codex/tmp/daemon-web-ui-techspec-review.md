Review the following TechSpec draft as a strict technical reviewer.

Context:

- Repository under review: `/Users/pedronauck/Dev/compozy/looper`
- Reference repository for structure/patterns: `/Users/pedronauck/dev/compozy/agh`
- Product/UI reference: `docs/design/daemon-mockup`
- This is a design-review task only. Do not propose code patches. Review for architectural fit, contradictions with the current codebase, missing contracts, scope drift, validation gaps, and any places where the spec claims a structure that is not realistic for the current repository.

Review goals:

1. Find concrete issues, contradictions, or missing design details.
2. Prefer high-signal findings over compliments.
3. Check the spec against actual current daemon/API structure in `looper`.
4. Check whether the AGH mirroring claims are sound and scoped correctly.
5. Check whether the testing and serving model are realistically specified.

Output requirements:

- Return one JSON object only.
- Use this exact shape:
  {
  "verdict": "approve" | "approve_with_changes" | "rewrite_needed",
  "summary": "short string",
  "findings": [
  {
  "severity": "high" | "medium" | "low",
  "section": "section name",
  "title": "short title",
  "issue": "what is wrong or missing",
  "recommendation": "concrete spec-level fix"
  }
  ],
  "strengths": ["short string"],
  "final_recommendation": "short string"
  }

TechSpec draft:

# Technical Specification: Daemon Web UI Runtime App for Compozy

## Executive Summary

This specification introduces the official daemon web UI for Compozy as a daemon-served React SPA, using the same runtime frontend structure and toolchain used in `/Users/pedronauck/dev/compozy/agh`, while implementing the product model and visual direction defined by `docs/design/daemon-mockup`. No `_prd.md` exists for this feature. This document is derived from the approved technical clarification sequence, the daemon architecture already accepted under `.compozy/tasks/daemon/`, direct exploration of the current daemon code, the local mockup, and the AGH runtime frontend structure.

The implementation keeps the daemon as the only control plane. The browser reads and mutates state only through daemon-owned REST and SSE contracts, with OpenAPI as the canonical web contract and generated types in the frontend. Production remains single-binary and local-first: the daemon serves the SPA at `/`, preserves the API at `/api`, and embeds `web/dist` with `go:embed`. The primary trade-off is deliberate cross-stack work in Go, frontend workspace tooling, and API contract generation in exchange for a coherent operator console that matches the mockup, starts with the daemon by default, and avoids split-brain state between browser, CLI, and workspace files.

Inference: the SPA operates against one active workspace at a time, selected from the daemon's workspace registry. This keeps the UI aligned with the mockup's workspace-centric shell while staying compatible with the daemon's existing multi-workspace architecture.

## System Architecture

### Component Overview

1. `Frontend workspace`
   - Add a Bun workspace rooted in the repository with `web/` and `packages/ui/`.
   - `web/` hosts the daemon SPA using React 19, TanStack Router, TanStack Query, Vite, Vitest, Storybook, Playwright, MSW, Tailwind CSS v4, shadcn, Zustand, Zod, and `openapi-fetch`.
   - `packages/ui/` hosts shared UI primitives, theme tokens, typography, and cross-route composition helpers.
   - The package structure mirrors AGH's runtime frontend slices, not its broader monorepo.

2. `Theme and visual system`
   - Reuse AGH's structural frontend architecture, not AGH's product visuals.
   - Tokens and typography are derived from `docs/design/daemon-mockup/colors_and_type.css`.
   - The default theme is dark-first with Compozy daemon branding, lime signal accents, mono metadata labels, and self-hosted display/mono fonts from the mockup assets.

3. `Embedded asset serving`
   - Add `web/embed.go` that embeds `web/dist`.
   - Extend `internal/api/httpapi` to serve exact static assets at `/`, bypass `/api`, and fall back to `index.html` for SPA routes.
   - Keep UDS API-only. Browser UI is served only from the daemon HTTP listener on `127.0.0.1`.

4. `Daemon web contract layer`
   - Extend `internal/api/core` with typed DTOs and handlers for dashboard, workflow overview, spec, memory, task board/detail, review detail, and richer run surfaces.
   - Publish an OpenAPI schema for the daemon HTTP API and generate typed frontend bindings in the same style used by AGH.

5. `Daemon read-model services`
   - Reuse `global.db` for workflow, task, review, and run summary projections.
   - Reuse `run.db` and existing run manager streaming for run detail and live event views.
   - Serve PRD, TechSpec, ADR, and memory content through daemon-side document readers that read canonical workspace files on the server. The browser never touches the filesystem directly.

6. `Operational action surfaces`
   - Support daemon-backed mutations required by the v1 operator console: sync, archive, run start, run cancel, and review-fix dispatch.
   - Keep PRD, TechSpec, task, and memory authoring outside the browser in v1.

### Route Model

The SPA route tree maps the mockup's screens into file-based TanStack Router routes:

- `/` dashboard
- `/workflows`
- `/workflows/$workflowSlug/spec`
- `/workflows/$workflowSlug/tasks`
- `/workflows/$workflowSlug/tasks/$taskId`
- `/runs`
- `/runs/$runId`
- `/reviews`
- `/reviews/$workflowSlug/$round/$issueId`
- `/memory`
- `/memory/$workflowSlug`

A global app shell provides sidebar navigation, breadcrumbs, command/search affordances, quick actions, and an active workspace selector. All workflow-scoped routes are resolved relative to the active workspace context.

### Data Flow

1. The browser requests `/` from the daemon HTTP listener.
2. `internal/api/httpapi` serves the embedded SPA bundle.
3. The root route resolves the active workspace from daemon-backed registry endpoints.
4. Route loaders and query hooks call the OpenAPI-generated client against `/api`.
5. Dashboard, workflow inventory, review lists, and run summaries read daemon projections from `global.db`.
6. Run detail and live status use daemon snapshot endpoints and SSE streams backed by `run.db` and the existing run manager.
7. Spec and memory pages fetch daemon document payloads; the daemon reads the canonical markdown files server-side and normalizes them into typed responses.
8. Operational actions post back to the daemon, then invalidate query state and rehydrate the affected views.
9. Storybook route stories use MSW to mock the same `/api` contracts for isolated review and UI regression coverage.

## Implementation Design

### Core Interfaces

```go
type WebDocumentService interface {
	WorkflowSpec(ctx context.Context, workspaceID, workflowSlug string) (WorkflowSpecDocument, error)
	WorkflowMemoryIndex(ctx context.Context, workspaceID, workflowSlug string) (WorkflowMemoryIndex, error)
	WorkflowMemoryFile(ctx context.Context, workspaceID, workflowSlug, fileName string) (MarkdownDocument, error)
	WorkflowTaskDetail(ctx context.Context, workspaceID, workflowSlug, taskID string) (TaskDetailPayload, error)
}
```

```go
type WorkflowSpecDocument struct {
	Workflow WorkflowRef      `json:"workflow"`
	PRD      *MarkdownDocument `json:"prd,omitempty"`
	TechSpec *MarkdownDocument `json:"techspec,omitempty"`
	ADRs     []ADRDocument     `json:"adrs"`
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
- SSE endpoints preserve cursor and reconnect semantics already used by the daemon transport layer

### Data Models

#### Repository and Package Topology

- root `package.json`, `tsconfig.json`, workspace scripts, and Bun lockfile
- `web/` for the SPA
- `packages/ui/` for reusable UI primitives and tokens
- no `packages/site` in this feature

#### Frontend Module Structure

- `web/src/routes/` for file-based route shells
- `web/src/components/` for app shell composition
- `web/src/systems/<domain>/` for domain modules
- `web/src/lib/` for shared runtime helpers and the generated API client
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
  - task metadata, latest task status, task memory excerpt, related run summary, live tail availability
- `RunDetailPayload`
  - run snapshot, provider/model/runtime config, job counters, token usage, timeline, live stream cursor
- `ReviewDetailPayload`
  - workflow slug, round, issue metadata, source location, body, proposed patch, discussion summary
- `WorkflowSpecDocument`
  - PRD, TechSpec, ADR list and markdown bodies
- `WorkflowMemoryIndex`
  - workflow memory file list, file metadata, active-task markers
- `MarkdownDocument`
  - path, title, updated at, raw markdown, rendered metadata

#### Source of Truth Rules

- `global.db`
  - workflow summaries, task/review/run indexes, queue state, registry state
- `run.db`
  - run events, transcript, live stream replay
- workspace filesystem
  - canonical PRD, TechSpec, ADR, and memory markdown documents
- browser
  - no source of truth; query cache only

### API Endpoints

The web UI uses daemon-owned resource endpoints. Existing routes are reused where they fit; missing read models are added in the same resource style.

| Method | Path                                                                            | Description                                                           |
| ------ | ------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `GET`  | `/api/daemon/status`                                                            | Daemon identity, version, uptime, listener info, workspace/run counts |
| `GET`  | `/api/daemon/health`                                                            | Ready/degraded state for dashboard health strip                       |
| `GET`  | `/api/workspaces`                                                               | Registered workspaces for active workspace selection                  |
| `POST` | `/api/workspaces/resolve`                                                       | Resolve or register the active workspace                              |
| `GET`  | `/api/workspaces/:workspace_id/dashboard`                                       | Dashboard aggregate payload for the active workspace                  |
| `GET`  | `/api/workspaces/:workspace_id/workflows`                                       | Workflow inventory for cards and sidebar                              |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug`                                 | Workflow overview payload                                             |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/spec`                            | PRD, TechSpec, and ADR read payloads                                  |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/memory`                          | Memory file index for one workflow                                    |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/memory/:file`                    | One memory notebook body                                              |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/tasks/board`                     | Kanban/list task payload                                              |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/tasks/:task_id`                  | Task detail payload including related run state                       |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/reviews`                         | Review round summary and issue list                                   |
| `GET`  | `/api/workspaces/:workspace_id/workflows/:slug/reviews/:round/issues/:issue_id` | Review issue detail payload                                           |
| `POST` | `/api/workspaces/:workspace_id/workflows/:slug/sync`                            | Explicit workflow sync                                                |
| `POST` | `/api/workspaces/:workspace_id/workflows/:slug/archive`                         | Archive one workflow                                                  |
| `POST` | `/api/workspaces/:workspace_id/workflows/:slug/runs`                            | Start a workflow run from the web UI                                  |
| `POST` | `/api/workspaces/:workspace_id/workflows/:slug/reviews/:round/fix-runs`         | Start a review-fix run                                                |
| `GET`  | `/api/runs`                                                                     | Cross-workflow run inventory                                          |
| `GET`  | `/api/runs/:run_id`                                                             | Stable run summary                                                    |
| `GET`  | `/api/runs/:run_id/snapshot`                                                    | Rich run detail snapshot                                              |
| `GET`  | `/api/runs/:run_id/stream`                                                      | SSE stream for live run updates                                       |
| `POST` | `/api/runs/:run_id/cancel`                                                      | Cancel a run                                                          |

Status conventions:

- `200` for synchronous reads
- `202` when a mutation starts a long-running daemon action
- `404` when workspace, workflow, task, review, or run is missing
- `409` for invalid concurrent operations such as duplicate run starts
- shared problem envelope for all non-success JSON responses

## Integration Points

No new runtime external services are introduced. The feature integrates with:

- the existing local daemon HTTP API
- the existing daemon run manager and databases
- the new local frontend toolchain and code generation workflow

OpenAPI code generation, Storybook, MSW, Vite, Vitest, and Playwright are build-time and test-time integrations, not new runtime platform dependencies.

## Impact Analysis

| Component                               | Impact Type | Description and Risk                                                                                                       | Required Action                                                      |
| --------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| root JS workspace config                | new         | Adds Bun workspace, scripts, TS config, and frontend build flow. Medium risk because repo currently has no JS workspace.   | Add workspace files and verification commands.                       |
| `web/`                                  | new         | Adds the daemon SPA and route/domain architecture. Medium risk because it introduces a second implementation stack.        | Create app shell, routes, systems, and generated client integration. |
| `packages/ui/`                          | new         | Adds shared UI primitives and tokens. Low to medium risk if boundaries are not enforced.                                   | Mirror AGH runtime UI package structure and mockup theme tokens.     |
| `internal/api/httpapi`                  | modified    | Gains embedded asset serving and SPA fallback routing. Medium risk because bad routing can shadow `/api`.                  | Add static FS loading, bypass rules, and tests.                      |
| `internal/api/core`                     | modified    | Gains richer web-facing DTOs and handlers. Medium risk because current contract is CLI-first.                              | Add document/dashboard/workflow endpoints and OpenAPI descriptions.  |
| `internal/daemon/*transport_service.go` | modified    | Must compose new read models from DB plus workspace documents. Medium risk because read models span multiple sources.      | Add workflow/spec/memory/review/task query services.                 |
| build and verify pipeline               | modified    | Must build web assets before embedding and enforce frontend verification. Medium risk because verify is currently Go-only. | Extend `make verify` with web build/lint/typecheck/test gates.       |
| OpenAPI/codegen path                    | new         | Adds generated web contract artifacts. Low to medium risk if codegen drifts.                                               | Add `codegen` and `codegen-check` workflow and CI enforcement.       |

## Testing Approach

### Unit Tests

- Vitest for `packages/ui` primitives, app shell behavior, route loaders, view-model hooks, and domain formatters
- generated client contract tests for success, empty payload, and problem-envelope handling
- Go unit tests for static asset resolution, SPA fallback bypass rules, and embedded bundle loading
- MSW-backed route stories and isolated component tests for loading, empty, success, degraded, and error states

Mocks are allowed only for isolated frontend states, Storybook, and explicit I/O boundaries. They do not replace real daemon integration.

### Integration Tests

- Go integration tests for daemon HTTP serving embedded assets while preserving `/api`
- Playwright against the daemon serving embedded assets, not only Vite dev mode
- seeded daemon integration flows for:
  - dashboard load
  - workflow inventory and drill-down
  - run detail and live stream
  - review detail and review-fix dispatch
  - spec and memory read views
  - workflow sync, archive, run start, and run cancel
- Storybook coverage for route-level and component-level mocked states using MSW

Final validation for this feature must rely on real daemon-served integration and end-to-end tests in addition to Vitest and Storybook.

## Development Sequencing

### Build Order

1. Add the Bun workspace, root frontend tooling, and `web/` plus `packages/ui/` package skeletons. Depends on no previous steps.
2. Implement `packages/ui` tokens, typography, shadcn primitives, and mockup-aligned shell components. Depends on step 1.
3. Add frontend build, lint, typecheck, test, and codegen scripts to the repo verification contract. Depends on step 1.
4. Add `web/embed.go`, daemon HTTP static serving, SPA fallback, and asset-serving tests. Depends on steps 1 and 3.
5. Add OpenAPI generation and the typed frontend client. Depends on steps 1 and 3.
6. Implement backend read-model endpoints for dashboard, workflow overview, spec, memory, task board/detail, and review detail. Depends on steps 4 and 5.
7. Build the SPA route tree, domain systems, query hooks, and operational actions against the typed client. Depends on steps 2, 5, and 6.
8. Add Storybook route stories and MSW handlers for all major UI states. Depends on steps 2, 5, and 7.
9. Add Playwright daemon-served E2E coverage and final verification wiring. Depends on steps 4, 6, 7, and 8.

### Technical Dependencies

- Bun workspace support checked into the repository
- OpenAPI generation workflow and checked-in generated artifacts policy
- frontend verification integrated into `make verify`
- seeded daemon test fixtures for Playwright and Go integration tests
- stable active-workspace selection policy in the web shell

## Monitoring and Observability

Operational visibility remains local-first and daemon-owned.

Key metrics:

- `daemon_http_web_requests_total`
- `daemon_http_web_asset_miss_total`
- `daemon_http_web_spa_fallback_total`
- `daemon_web_api_errors_total`
- `daemon_web_run_stream_reconnect_total`
- `daemon_web_action_latency_seconds`

Structured log fields:

- `request_id`
- `workspace_id`
- `workflow_slug`
- `task_id`
- `review_round`
- `issue_id`
- `run_id`
- `route_id`
- `asset_path`
- `action`

Health and degradation signals:

- embedded bundle missing or unreadable
- SPA fallback serving failures
- document read failures for spec or memory
- repeated run stream reconnect loops
- OpenAPI/client version drift detected during build or startup checks

V1 does not require an external alerting system. Degraded states surface through daemon health endpoints, logs, and the UI status strip.

## Technical Considerations

### Key Decisions

- Decision: mirror only AGH's runtime frontend slices, not the broader monorepo.
  - Rationale: it satisfies the user's structural requirement without dragging in unrelated workspaces.
  - Trade-off: less one-to-one repo parity with AGH.
  - Alternatives rejected: single-package SPA, broad monorepo mirror.

- Decision: serve the SPA from the daemon's existing HTTP listener and embed assets into the binary.
  - Rationale: it preserves the single-binary daemon model and matches AGH's proven runtime posture.
  - Trade-off: backend owns static fallback routing and build ordering.
  - Alternatives rejected: separate web listener, external frontend server in production.

- Decision: keep the browser daemon-only and contract-driven with OpenAPI-generated types.
  - Rationale: it avoids split-brain state and keeps CLI and web aligned on one control plane.
  - Trade-off: the daemon must grow richer read models.
  - Alternatives rejected: hybrid browser filesystem reads, hand-maintained frontend types.

- Decision: keep spec and memory read-only in v1.
  - Rationale: it delivers the operator console implied by the mockup without taking on browser authoring complexity.
  - Trade-off: some artifact interactions remain CLI-driven.
  - Alternatives rejected: observability-only UI, full browser editing from day one.

- Decision: require full frontend verification including Storybook and MSW.
  - Rationale: this feature introduces new UI, new asset serving, and new daemon APIs at once.
  - Trade-off: heavier CI and local verification.
  - Alternatives rejected: Vitest-only confidence, minimal E2E.

### Known Risks

- The current daemon API is CLI-first, so the new workflow/spec/memory/task/review read models may sprawl unless handler boundaries stay tight.
- Mirroring AGH's structure without mirroring its product semantics could accidentally leak AGH naming or UX assumptions into Compozy.
- Server-side document reads for spec and memory can become inconsistent if cache invalidation and watcher behavior are underspecified.
- Frontend workspace adoption can slow verification until the build graph and cache behavior are stable.
- The assumed single active workspace session model may need refinement if operators routinely work across many registered workspaces in one browser session.

## Architecture Decision Records

- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — Adopt only the AGH runtime-facing frontend slices and keep broader workspaces out of scope.
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — Keep production single-origin and single-binary with embedded assets at `/`.
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — Make the daemon the only browser integration boundary and OpenAPI the canonical web contract.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — Deliver the operator console without browser editing.
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md) — Put strong frontend and daemon-served validation in the primary scope.
