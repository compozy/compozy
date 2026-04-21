---
status: pending
title: OpenAPI Contract Artifact and Typed Client Codegen
type: infra
complexity: high
dependencies:
  - task_01
---

# OpenAPI Contract Artifact and Typed Client Codegen

## Overview

This task creates the checked-in browser contract for the daemon web UI and wires the typed client generation flow around it. It ensures the frontend consumes a stable OpenAPI artifact through generated types and `openapi-fetch` rather than hand-maintained REST types.

<critical>
- ALWAYS READ `_techspec.md` and ADRs before starting; there is no `_prd.md` for this feature
- REFERENCE the TechSpec sections "OpenAPI Generation Contract", "API Endpoints", and "Development Sequencing"
- FOCUS ON "WHAT" — define the contract and deterministic codegen workflow, not the full handler implementation
- MINIMIZE CODE — keep server implementation ownership in Go and avoid generating server stubs
- TESTS REQUIRED — contract generation and drift detection must be executable, not manual
</critical>

<requirements>
1. MUST add `openapi/compozy-daemon.json` as the checked-in browser-facing contract artifact.
2. MUST add deterministic `codegen` and `codegen-check` commands that generate `web/src/generated/compozy-openapi.d.ts`.
3. MUST create the base typed transport client in `web/src/lib/api-client.ts` using `openapi-fetch`.
4. MUST keep Go handlers as the implementation source of truth and MUST NOT introduce OpenAPI-generated server stubs.
5. SHOULD add contract checks that fail when regenerating the types produces an unexpected diff.
</requirements>

## Subtasks
- [ ] 2.1 Add the checked-in OpenAPI document at the repository root for the daemon browser contract.
- [ ] 2.2 Wire root and package-level codegen commands for generate and drift-check workflows.
- [ ] 2.3 Create the generated typings output path and the typed web API client wrapper.
- [ ] 2.4 Add automated checks proving the generated output stays in sync with the checked-in contract.
- [ ] 2.5 Document the codegen contract in the package/build scripts expected by later tasks.

## Implementation Details

See the TechSpec sections "API Endpoints", "OpenAPI Generation Contract", and "Technical Dependencies". This task sets the canonical typed integration boundary for the browser and should be complete before the frontend route/domain slices start binding to daemon data.

### Relevant Files
- `openapi/compozy-daemon.json` — new checked-in browser contract artifact defined by the TechSpec.
- `package.json` — root command surface for `codegen` and `codegen-check`.
- `web/package.json` — app-level scripts that must route through the shared codegen contract.
- `web/src/generated/compozy-openapi.d.ts` — generated typings output path mandated by the TechSpec.
- `web/src/lib/api-client.ts` — typed client wrapper around `openapi-fetch`.
- `web/src/lib/api-contract.ts` — contract helper surface that should mirror AGH's typed client organization.
- `internal/api/core/interfaces.go` — current transport DTO definitions that the contract must reflect accurately.

### Dependent Files
- `internal/api/core/routes.go` — later handler work must align with the paths and shapes introduced here.
- `internal/api/core/handlers.go` — later read-model handler work must satisfy the browser contract defined here.
- `web/src/routes/` — later route/domain tasks depend on the generated client and typings created here.
- `web/src/systems/` — later query hooks and domain state should consume the typed client established here.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — establishes OpenAPI as the browser contract source.
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — requires the frontend slices to consume a typed client in the AGH style.

## Deliverables
- Checked-in `openapi/compozy-daemon.json` describing the browser-facing daemon contract.
- Deterministic `codegen` and `codegen-check` commands wired into the workspace scripts.
- Generated OpenAPI typings and a base `openapi-fetch` web client.
- Unit tests and contract drift checks with 80%+ coverage **(REQUIRED)**
- Integration checks proving the codegen workflow is reproducible from a clean checkout **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] OpenAPI generation produces the expected typings output under `web/src/generated/`.
  - [ ] `codegen-check` fails when the generated types would differ from the checked-in output.
  - [ ] The typed client wrapper exposes the expected base configuration for later route/domain consumers.
- Integration tests:
  - [ ] Running the codegen workflow from the repository root succeeds on a clean checkout.
  - [ ] Web package scripts that depend on codegen can run without manual presteps.
  - [ ] Contract drift is detected automatically instead of relying on manual inspection.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon browser contract exists as a checked-in OpenAPI artifact
- The frontend has a deterministic typed client generation path
- Later route/domain tasks can consume daemon APIs without writing manual REST types
