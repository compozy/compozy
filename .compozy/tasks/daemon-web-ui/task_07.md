---
status: pending
title: Active Workspace, Browser Security, and SSE Compatibility
type: backend
complexity: critical
dependencies:
  - task_06
---

# Active Workspace, Browser Security, and SSE Compatibility

## Overview

This task hardens the browser-facing daemon API by formalizing active-workspace semantics, localhost browser security, and the run-stream SSE contract. It is the highest-risk backend slice because it changes how browser requests are scoped and because stream compatibility affects existing run consumers as well as the new web UI.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_06.md` before starting
- REFERENCE the TechSpec sections "Active Workspace Model", "Streaming Contract", "Security", and "Known Risks"
- FOCUS ON "WHAT" — define the browser-only safety and context rules without regressing CLI/UDS callers
- MINIMIZE CODE — keep HTTP/browser-specific behavior out of the UDS path wherever possible
- TESTS REQUIRED — security, workspace scoping, and SSE compatibility must be covered with executable tests
</critical>

<requirements>
1. MUST support the browser's single-workspace-per-tab model using `X-Compozy-Workspace-ID`, including stale or missing workspace handling that returns `412`.
2. MUST preserve existing non-browser daemon flows that currently pass workspace identity via query/body rather than the new browser header.
3. MUST add strict Host/Origin validation and same-origin CSRF protection for mutating browser endpoints on the HTTP transport.
4. MUST formalize the SSE contract for run streams, including event IDs, heartbeat cadence, overflow signaling, and reconnect semantics.
5. SHOULD add compatibility coverage for `internal/api/client` and `pkg/compozy/runs` so the new SSE semantics do not silently break existing consumers.
</requirements>

## Subtasks
- [ ] 7.1 Add active-workspace resolution and stale-workspace failure handling for browser requests.
- [ ] 7.2 Preserve current CLI/UDS workspace-scoping flows while introducing the browser header contract.
- [ ] 7.3 Add Host/Origin validation and CSRF protection to the browser-facing HTTP transport.
- [ ] 7.4 Implement the explicit SSE heartbeat, overflow, and reconnect semantics defined by the TechSpec.
- [ ] 7.5 Add compatibility and security tests covering browser, client, and run-reader behaviors.

## Implementation Details

See the TechSpec sections "Active Workspace Model", "Streaming Contract", "Security", and "Testing Approach". This task should isolate browser-only trust and scoping behavior in the HTTP path while keeping the shared daemon transport contract compatible for existing clients.

### Relevant Files
- `internal/api/core/middleware.go` — shared middleware layer that should enforce workspace-context and transport rules.
- `internal/api/core/sse.go` — shared SSE transport implementation that must adopt the explicit stream semantics.
- `internal/api/core/handlers.go` — handler behavior that must translate stale/missing workspace context cleanly.
- `internal/api/httpapi/server.go` — HTTP transport construction point where browser-only middleware is applied.
- `internal/api/client/runs.go` — existing daemon client path that must remain compatible with any stream changes.
- `pkg/compozy/runs/remote_watch.go` — public run-reader compatibility surface that the stream changes must not regress.

### Dependent Files
- `internal/api/httpapi/static.go` — later static serving depends on the browser HTTP middleware chain being correct.
- `web/src/routes/` — frontend route loaders and actions depend on stable `412`, CSRF, and SSE behavior.
- `web/src/lib/api-client.ts` — browser client code depends on the active-workspace and mutation security contract defined here.
- `web/e2e/` — later browser E2E coverage depends on the SSE and security behavior being explicit and stable.

### Related ADRs
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — makes the browser-facing localhost listener a real runtime boundary.
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — defines REST/SSE as the browser integration boundary.

## Deliverables
- Browser-aware workspace context handling using `X-Compozy-Workspace-ID`.
- Host/Origin validation and same-origin CSRF protection for browser mutations.
- Explicit SSE heartbeat, overflow, and reconnect semantics compatible with existing run consumers.
- Unit tests for workspace/security/SSE behavior with 80%+ coverage **(REQUIRED)**
- Integration tests proving browser HTTP hardening does not regress current client flows **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Missing or stale `X-Compozy-Workspace-ID` produces the expected typed `412` failure for browser routes.
  - [ ] Unexpected Host or Origin headers are rejected on the HTTP transport.
  - [ ] Mutating browser endpoints enforce the expected CSRF contract.
  - [ ] SSE streams emit the expected heartbeat, cursor, and overflow semantics.
- Integration tests:
  - [ ] Existing daemon clients that do not use the browser header continue to function correctly.
  - [ ] `pkg/compozy/runs` consumers can reconnect and replay correctly after the stream contract changes.
  - [ ] Browser mutation and streaming flows work through the HTTP transport without leaking browser-only policy into UDS.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Browser requests have explicit workspace, security, and stream semantics
- Existing CLI/UDS and run-reader consumers remain compatible
- Later frontend and E2E tasks can depend on stable browser-facing daemon behavior
