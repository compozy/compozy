---
status: pending
title: Embedded SPA Serving and HTTP Fallback
type: backend
complexity: high
dependencies:
  - task_01
  - task_07
---

# Embedded SPA Serving and HTTP Fallback

## Overview

This task makes the web UI a real daemon-served runtime app by embedding the built SPA into the binary and serving it from the existing localhost HTTP listener. It is responsible for exact asset serving, `/api` preservation, and SPA fallback behavior that mirrors the AGH serving model.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_07.md` before starting
- REFERENCE the TechSpec sections "Embedded asset serving", "Data Flow", and "Testing Approach"
- FOCUS ON "WHAT" — make the daemon serve the SPA correctly without changing the single-listener topology
- MINIMIZE CODE — follow the proven AGH static-serving structure instead of inventing a custom router layer
- TESTS REQUIRED — asset serving, `/api` preservation, and fallback behavior must be covered with integration tests
</critical>

<requirements>
1. MUST add `web/embed.go` to embed the built `web/dist` assets into the daemon binary.
2. MUST serve exact static assets at `/` and MUST fall back to `index.html` for SPA routes while preserving `/api`.
3. MUST keep the UDS transport API-only and MUST NOT route browser asset traffic through it.
4. MUST keep the daemon's existing localhost HTTP listener as the single runtime origin for both UI and API.
5. SHOULD add transport/integration tests proving API routes are not shadowed by static fallback behavior.
</requirements>

## Subtasks
- [ ] 8.1 Add the embedded web asset package and binary embed contract for `web/dist`.
- [ ] 8.2 Implement HTTP static serving and SPA fallback behavior that bypasses `/api`.
- [ ] 8.3 Wire the daemon host/runtime so the embedded SPA is served from the existing HTTP listener.
- [ ] 8.4 Keep the UDS server explicitly API-only while the browser UI lives on HTTP.
- [ ] 8.5 Add integration tests covering asset resolution, fallback, and `/api` preservation.

## Implementation Details

See the TechSpec sections "Embedded asset serving", "Data Flow", "Impact Analysis", and "Testing Approach". This task should make the web UI the default daemon-served app in production-like mode, without yet expanding repository verification or browser E2E coverage.

### Relevant Files
- `web/embed.go` — new embedded asset package for the daemon UI bundle.
- `internal/api/httpapi/server.go` — HTTP transport construction where static serving must be wired in.
- `internal/api/httpapi/routes.go` — shared HTTP registration path that must preserve `/api` while adding SPA support.
- `internal/api/httpapi/static.go` — new static-asset and SPA-fallback serving logic modeled after AGH.
- `internal/daemon/host.go` — daemon host startup path that owns the HTTP server used for the embedded SPA.
- `internal/api/httpapi/transport_integration_test.go` — existing HTTP/UDS parity test surface that should grow with serving coverage.

### Dependent Files
- `Makefile` — later build/verify wiring depends on the embed contract created here.
- `.github/workflows/ci.yml` — later CI wiring depends on the embedded web build existing as part of the repo contract.
- `web/playwright.config.ts` — later daemon-served E2E work depends on the embedded asset serving lane introduced here.
- `.compozy/tasks/daemon-web-ui/qa/` — later QA tasks depend on the UI being served by the real daemon HTTP runtime.

### Related ADRs
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — defines the single-listener embed model this task must implement.
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — makes `web/` the embedded runtime app rather than a separate site.

## Deliverables
- `web/embed.go` and the embedded bundle contract for `web/dist`.
- HTTP static serving and SPA fallback behavior preserving `/api`.
- Daemon host/runtime wiring that serves the embedded UI from the existing HTTP listener.
- Unit tests and integration tests for asset serving with 80%+ coverage **(REQUIRED)**
- Integration tests proving `/api` preservation and UDS/API separation **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Embedded asset resolution finds built files and returns a clear failure when the bundle contract is broken.
  - [ ] Static routing bypasses `/api` and other explicit daemon paths before applying SPA fallback.
  - [ ] SPA fallback serves `index.html` only for non-asset browser routes.
- Integration tests:
  - [ ] The daemon HTTP listener serves the embedded UI and still responds correctly on `/api`.
  - [ ] Deep-link browser routes resolve through SPA fallback without shadowing API endpoints.
  - [ ] UDS continues to expose only the daemon API surface and not web assets.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon serves the embedded SPA at `/` while preserving `/api`
- The web UI is production-like and daemon-served by default
- Later verify, Playwright, and QA tasks can exercise the real embedded-app topology
