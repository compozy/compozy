---
status: pending
title: Runs Console and Operational Run Controls
type: frontend
complexity: high
dependencies:
  - task_02
  - task_03
  - task_06
  - task_07
---

# Runs Console and Operational Run Controls

## Overview

This task implements the run-centric operator surfaces of the daemon web UI, including the run list, run detail, live watch, and browser-triggered run control actions. It is the frontend slice most coupled to the explicit SSE contract and needs to prove that real-time run state is understandable and controllable from the browser.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_02.md` through `task_07.md` before starting
- REFERENCE the TechSpec sections "Streaming Contract", "Route Model", "API Endpoints", and "Testing Approach"
- FOCUS ON "WHAT" — implement the operator-facing run surfaces, not unrelated dashboard or document pages
- MINIMIZE CODE — consume the typed client and stream semantics directly rather than layering a second custom protocol
- TESTS REQUIRED — route, stream, reconnect, and action behavior must be covered with executable tests
</critical>

<requirements>
1. MUST implement `/runs` and `/runs/$runId` using the typed REST and SSE daemon contracts.
2. MUST support run watch/reconnect behavior, including heartbeat, overflow, and snapshot refresh semantics defined by the TechSpec.
3. MUST expose supported run control actions from the browser, including start and cancel flows where the daemon contract allows them.
4. MUST present loading, degraded, empty, and error states clearly for live run views.
5. SHOULD include tests that specifically cover stream reconnect and cancellation flows.
</requirements>

## Subtasks
- [ ] 10.1 Build the run list and run detail routes with typed loader/query behavior.
- [ ] 10.2 Implement live run streaming with reconnect and overflow handling.
- [ ] 10.3 Add the supported operator actions for starting and canceling runs from the browser.
- [ ] 10.4 Render loading, degraded, empty, and error states for run-centric views.
- [ ] 10.5 Add route, hook, and integration tests for run rendering and stream behavior.

## Implementation Details

See the TechSpec sections "Streaming Contract", "Data Flow", "Route Model", and "Testing Approach". This task should prove the browser can operate comfortably against the daemon's run model using the typed snapshot and stream contracts already defined on the backend.

### Relevant Files
- `web/src/routes/_app/runs.tsx` — route for the cross-workflow run inventory view.
- `web/src/routes/_app/runs.$runId.tsx` — route for run detail, live watch, and operational controls.
- `web/src/systems/runs/` — run domain logic for queries, streaming, view models, and action handling.
- `web/src/lib/api-client.ts` — typed browser client used for snapshot, list, and mutation requests.
- `internal/api/core/sse.go` — backend stream contract reference that this UI slice must honor.
- `pkg/compozy/runs/remote_watch.go` — compatibility reference for reconnect/stream expectations already present in the codebase.

### Dependent Files
- `web/src/storybook/` — later story coverage depends on stable run list/detail states from this task.
- `web/e2e/` — later browser E2E work depends on deterministic run flows and live stream behavior.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/` — later QA planning depends on the run flows implemented here.
- `web/src/routes/_app/index.tsx` — dashboard integrations may surface links or summaries into the run console implemented here.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — defines the run REST/SSE browser contract.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — puts live run operations squarely in v1 scope.

## Deliverables
- Run list and run detail routes consuming typed daemon REST/SSE contracts.
- Browser run watch/reconnect handling aligned with the explicit stream contract.
- Supported run control actions with clear operator feedback.
- Unit tests for run queries/hooks/view models with 80%+ coverage **(REQUIRED)**
- Integration tests for live run views, reconnect, and cancel/start flows **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Run list and run detail loaders consume the typed client correctly.
  - [ ] Stream hooks handle heartbeat, overflow, and reconnect semantics correctly.
  - [ ] Run-control actions update local query state and user feedback appropriately.
- Integration tests:
  - [ ] `/runs` and `/runs/$runId` render correctly against real daemon responses.
  - [ ] Stream reconnect after overflow or disconnect refreshes the snapshot and resumes correctly.
  - [ ] Supported start/cancel run actions work end to end through the daemon contract.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The browser can inspect and operate daemon runs through real REST/SSE contracts
- Live run state remains comprehensible through reconnects and degraded conditions
- Later Storybook, E2E, and QA tasks can rely on stable run surfaces
