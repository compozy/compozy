# Daemon Web Dev Proxy Design

Date: 2026-04-21
Status: Approved in brainstorming
Scope: local development UX for the daemon-served web UI

## Context

The current local flow couples frontend iteration to the embedded production bundle:

- `web/` changes require `make build`
- the daemon must be stopped and started again
- the browser only sees updated assets after rebuilding `web/dist` and re-embedding them into the binary

That is the right production posture, but it is poor development ergonomics for frontend work.

The user explicitly asked to preserve a single browser URL in local development instead of exposing a separate Vite URL.

## Goals

- Keep the browser pointed at a single daemon URL during development.
- Enable hot reload for `web/` changes without rebuilding the Go binary.
- Preserve the existing production topology:
  - `/api` handled by the daemon
  - `/` and SPA routes served from the embedded bundle
- Reuse Turborepo for parallel local process orchestration.

## Non-goals

- Replace the embedded bundle production flow.
- Introduce a second public development URL as the primary path.
- Add a background process manager dependency just for local DX.
- Solve Go backend live reload in the same change.

## Approaches Considered

### 1. Watch `web/dist` and serve assets from disk in dev

Pros:

- minimal server-side behavior changes
- keeps one browser URL

Cons:

- no real HMR, only rebuild + refresh
- still tied to a build output directory
- weaker developer feedback loop than Vite

### 2. Recommended: daemon dev reverse proxy to Vite

Pros:

- preserves one browser URL
- keeps `/api` owned by the daemon
- delivers real Vite HMR
- production behavior remains unchanged when dev mode is off

Cons:

- requires a dedicated dev fallback handler in `httpapi`
- must correctly proxy websocket upgrades for HMR

### 3. Rebuild and restart the daemon automatically on file changes

Pros:

- smallest conceptual change

Cons:

- keeps the slowest part of the current loop
- disrupts daemon state on every frontend edit
- still not hot reload

## Approved Design

### Runtime routing

The daemon remains the single browser entrypoint in both prod and dev.

- Production mode:
  - `/api/**` stays on daemon handlers
  - non-API routes use the embedded `web/dist` bundle
- Development mode:
  - `/api/**` stays on daemon handlers
  - non-API routes are reverse proxied to the Vite dev server

The switch is controlled by a development-only daemon option and environment variable.

### Transport shape

`internal/api/httpapi` gains a dev proxy fallback handler alongside the existing embedded static fallback handler.

- `Server.finalize()` chooses one fallback mode:
  - embedded static handler
  - Vite dev proxy handler
- `RegisterRoutes(...)` keeps the shared API routes unchanged
- `NoRoute(...)` remains the single integration point for SPA/static fallback behavior

### Development orchestration

The repository gets a local dev workflow that runs:

- `web` via Vite on `127.0.0.1:3000`
- the compiled daemon binary via `./bin/compozy daemon start --foreground`

The daemon process sets:

- `COMPOZY_DAEMON_HTTP_PORT=2123`
- `COMPOZY_WEB_DEV_PROXY=http://127.0.0.1:3000`

To avoid colliding with the user's normal home-scoped daemon, the default dev flow runs the daemon with an isolated `HOME` rooted under the repository `.tmp/`, while `make dev-global` reuses the user's real `HOME`.

### Validation

The change requires:

- unit tests for dev proxy target validation and routing behavior
- CLI tests for dev proxy option propagation
- repository verification with `make verify`
