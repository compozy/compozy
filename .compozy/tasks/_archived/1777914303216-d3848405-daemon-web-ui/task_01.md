---
status: completed
title: Bun Workspace Expansion and Web Package Skeletons
type: infra
complexity: high
dependencies: []
---

# Bun Workspace Expansion and Web Package Skeletons

## Overview

This task expands the repository's existing Bun/Turbo tooling so the daemon web UI can live in first-class `web/` and `packages/ui/` workspaces. It establishes the minimal package, config, and bootstrap structure required for later web, OpenAPI, embed, and CI work without yet implementing product routes or daemon UI behavior.

<critical>
- ALWAYS READ `_techspec.md` and the ADRs before starting; there is no `_prd.md` for this feature
- REFERENCE the TechSpec sections "Component Overview", "Data Models", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" — create the workspace and package seams that later tasks depend on, not the full app
- MINIMIZE CODE — prefer adapting the existing root Bun/Turbo setup over inventing a parallel JS toolchain
- TESTS REQUIRED — every config or package bootstrap change must be covered by executable checks
</critical>

<requirements>
1. MUST expand the root workspace configuration so `web/` and `packages/ui/` are supported alongside the existing `sdk/*` packages.
2. MUST create the minimal package manifests, TypeScript configs, and Vite-compatible bootstrap files needed for `web/` and `packages/ui/` to install, typecheck, and build.
3. MUST preserve the current root Bun/Turbo behavior for the SDK packages and avoid breaking existing scripts.
4. MUST establish the initial placeholder `web/dist/` contract required for later embed/build work on fresh checkouts.
5. SHOULD mirror AGH's runtime-facing package boundaries and naming conventions where they fit this repository.
</requirements>

## Subtasks
- [x] 1.1 Expand the root Bun workspace and Turbo configuration to include `web/` and `packages/ui/`.
- [x] 1.2 Create the initial package manifests and TypeScript config files for the daemon SPA and shared UI package.
- [x] 1.3 Add the basic Vite/bootstrap files required for later route and component work.
- [x] 1.4 Introduce the initial `web/dist/` placeholder and package-level build entrypoints expected by later embed work.
- [x] 1.5 Add checks proving the new workspaces install and resolve correctly without regressing existing SDK packages.

## Implementation Details

See the TechSpec sections "Component Overview", "Data Models", "Impact Analysis", and "Development Sequencing". This task establishes only the repository/runtime scaffolding for the frontend slice; route trees, API contracts, static serving, and verification expansion belong to later tasks.

### Relevant Files
- `package.json` — current Bun workspace root that must grow beyond `sdk/*`.
- `turbo.json` — current task graph that needs `web/` and `packages/ui/` participation.
- `tsconfig.json` — root TS entrypoint that currently only includes root-level config consumers.
- `tsconfig.base.json` — shared strict TS defaults that the new packages should inherit.
- `web/package.json` — new daemon SPA manifest aligned with the AGH runtime app pattern.
- `packages/ui/package.json` — new shared UI package manifest aligned with AGH's reusable UI boundary.

### Dependent Files
- `openapi/compozy-daemon.json` — later codegen task depends on the workspace foundation created here.
- `web/embed.go` — later embed/static serving work depends on the `web/` package existing cleanly.
- `Makefile` — later verify/build expansion depends on stable package entrypoints from this task.
- `.github/workflows/ci.yml` — later CI wiring depends on a stable workspace/package layout.

### Related ADRs
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — defines the package split this task must establish.
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — makes `web/` a runtime app rather than a separate site workspace.

## Deliverables
- Expanded root Bun workspace and Turbo configuration covering `web/` and `packages/ui/`.
- Initial `web/` and `packages/ui/` package manifests and TS/Vite bootstrap files.
- Placeholder `web/dist/` contract suitable for later embed/build work.
- Unit tests or executable config checks proving workspace resolution with 80%+ coverage **(REQUIRED)**
- Integration checks proving the new packages install/build without breaking existing SDK workspaces **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Root workspace configuration includes `web/` and `packages/ui/` without dropping the current `sdk/*` entries.
  - [x] `web/` and `packages/ui/` inherit the repository's strict TypeScript defaults cleanly.
  - [x] Package entrypoints and exports resolve correctly from the root workspace graph.
- Integration tests:
  - [x] Fresh Bun install succeeds with the expanded workspace layout.
  - [x] Root build/typecheck commands can discover the new packages without regressing the existing SDK packages.
  - [x] Fresh checkout `go build` is not blocked by a missing `web/dist/` path.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The repository supports `web/` and `packages/ui/` as first-class workspaces
- Existing SDK packages still resolve and build through the root workspace contract
- Later frontend, embed, and CI tasks can assume a stable workspace/package layout
