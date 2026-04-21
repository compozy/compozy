---
status: pending
title: Bun-Aware Verify/CI Wiring and Daemon-Served Playwright E2E
type: test
complexity: critical
dependencies:
  - task_01
  - task_02
  - task_08
  - task_09
  - task_10
  - task_11
  - task_12
  - task_13
---

# Bun-Aware Verify/CI Wiring and Daemon-Served Playwright E2E

## Overview

This task turns the new frontend stack into a real repository contract by extending local verification, CI, and end-to-end browser coverage. It ensures the daemon web UI is built, tested, and exercised as part of the same repo gate that currently covers only Go code, and it establishes Playwright against daemon-served embedded assets rather than Vite-only dev mode.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_01.md`, `task_02.md`, `task_08.md` through `task_13.md` before starting
- REFERENCE the TechSpec sections "Testing Approach", "Development Sequencing", "Technical Dependencies", and "Impact Analysis"
- FOCUS ON "WHAT" — expand the repository verification contract and add durable browser E2E coverage, not ad hoc local scripts
- MINIMIZE CODE — wire the new frontend lane into existing repo commands and CI rather than creating a parallel unofficial gate
- TESTS REQUIRED — the repo gate, CI path filters, and Playwright daemon-served E2E flows must all be executable
</critical>

<requirements>
1. MUST extend `make build` and `make verify` so the repo gate includes Bun/bootstrap checks plus web lint, typecheck, test, and build before Go verification.
2. MUST update CI so web, package, OpenAPI, and build-script changes trigger the correct verification lane.
3. MUST add Playwright configuration and daemon-served E2E coverage that runs against the embedded-asset topology, not only a Vite dev server.
4. MUST preserve the fresh-checkout build behavior required by the `web/dist/` placeholder contract.
5. SHOULD include smoke coverage for the critical public browser flows required by the TechSpec so later QA tasks consume an existing E2E harness rather than inventing one.
</requirements>

## Subtasks
- [ ] 14.1 Extend the local repository gate with Bun-aware build and verify behavior.
- [ ] 14.2 Update CI path filters and job steps so frontend and contract changes are validated automatically.
- [ ] 14.3 Add Playwright configuration and daemon-served browser harness setup.
- [ ] 14.4 Implement or update critical smoke E2E specs for the public browser flows.
- [ ] 14.5 Add checks proving fresh-checkout build and embedded-asset assumptions still hold.

## Implementation Details

See the TechSpec sections "Testing Approach", "Development Sequencing", "Technical Dependencies", and "Impact Analysis". This task should be the point where the daemon web UI becomes part of the repository's official verification contract, including production-like browser automation.

### Relevant Files
- `Makefile` — current repo gate that must expand beyond Go-only verification.
- `.github/workflows/ci.yml` — current CI job and path-filter contract that must include frontend/openapi/build changes.
- `package.json` — root script surface for Bun/Turbo/web verification commands.
- `web/vite.config.ts` — app build configuration that must participate in the official verification lane.
- `web/vitest.config.ts` — app test configuration that must participate in the official verification lane.
- `packages/ui/vitest.config.ts` — shared UI package test configuration that must be wired into repo verification.
- `web/playwright.config.ts` — browser E2E harness configuration for daemon-served flows.
- `web/e2e/` — end-to-end browser specs covering the critical daemon web UI flows.
- `web/dist/` — fresh-checkout placeholder contract that must remain compatible with build and embed assumptions.

### Dependent Files
- `.compozy/tasks/daemon-web-ui/qa/test-plans/` — later QA planning depends on an existing verification and E2E harness.
- `.compozy/tasks/daemon-web-ui/qa/verification-report.md` — later QA execution depends on the repo gate defined here.
- `web/src/routes/` — browser E2E coverage depends on the stability of the routes delivered in earlier frontend tasks.
- `internal/api/httpapi/static.go` — daemon-served E2E coverage depends on the embedded serving behavior implemented earlier.

### Related ADRs
- [ADR-002: Serve the Embedded SPA from the Daemon's Existing HTTP Listener](adrs/adr-002.md) — requires E2E to validate the daemon-served embedded topology.
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md) — makes this verification lane part of core scope rather than optional follow-up.

## Deliverables
- Expanded `make build` and `make verify` including the frontend lane.
- Updated CI filters and jobs covering web, package, OpenAPI, and build-script changes.
- Playwright configuration and daemon-served E2E coverage for critical browser flows.
- Unit tests and configuration checks with 80%+ coverage **(REQUIRED)**
- Integration and E2E tests proving the daemon-served browser flows under the official repo gate **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Root scripts and Makefile steps run the expected frontend verification commands in the correct order.
  - [ ] CI path filters include the frontend, OpenAPI, and workspace-config surfaces introduced by this feature.
  - [ ] Playwright configuration targets the daemon-served runtime assumptions correctly.
- Integration tests:
  - [ ] `make verify` runs the Bun-aware frontend and Go verification lanes end to end.
  - [ ] CI can detect and run the correct verification lane when frontend or OpenAPI files change.
  - [ ] Playwright smoke specs pass against daemon-served embedded assets for the critical browser flows.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI is part of the repository's official verification and CI contract
- Browser E2E runs against the real daemon-served topology
- Later QA planning and execution can consume an existing, repo-supported browser harness
