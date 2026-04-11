---
status: completed
title: TypeScript SDK, starter templates, and author documentation
type: docs
complexity: high
dependencies:
  - task_14
---

# Task 15: TypeScript SDK, starter templates, and author documentation

## Overview
Ship the TypeScript SDK `@compozy/extension-sdk` for authors who want to write Compozy extensions in TypeScript or JavaScript, plus four starter templates that cover the most common extension archetypes, plus the author-facing documentation that explains how to build, test, install, and ship an extension. This task is the final piece that makes the extensibility system usable by the outside world.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Development Sequencing steps 17-19 and the SDK shape mirrored from task 14.
</critical>

<requirements>
- MUST publish a TypeScript package named `@compozy/extension-sdk` that mirrors the shape of the Go SDK from task 14.
- MUST implement a stdin/stdout JSON-RPC transport in TypeScript using line-delimited UTF-8 JSON.
- MUST expose an `Extension` class with handler registration APIs for `execute_hook`, `on_event`, `health_check`, and `shutdown`.
- MUST expose a `HostAPI` client covering every Host API method from `_protocol.md` section 5.2 with typed request/response interfaces.
- MUST ship type definitions (`.d.ts`) for every public type used by authors.
- MUST ship four starter templates: lifecycle observer, prompt decorator, review provider (declarative), skill pack (declarative).
- MUST provide a scaffolding CLI `npx @compozy/create-extension <name>` that copies a template into a new directory and runs `npm install` / `go mod init` as appropriate.
- MUST ship author documentation at `.compozy/docs/extensibility/` covering: getting started, architecture overview, hook reference (generated from `_protocol.md` section 6.5), Host API reference, capability reference, trust and enablement model, testing with the harness, and a migration guide from early prototypes.
- MUST include a hello-world example for both Go and TypeScript that a new user can copy-paste and run in under five minutes.
- MUST match the protocol version (`"1"`) and keep package versioning in lockstep with the Compozy runtime.
- MUST NOT ship any secret keys, private tokens, or user-specific state in templates or docs.
</requirements>

## Subtasks
- [x] 15.1 Create the `@compozy/extension-sdk` TypeScript package with `package.json`, TypeScript config, and build setup.
- [x] 15.2 Implement the JSON-RPC stdio transport, initialize handshake client, and handler dispatch core.
- [x] 15.3 Implement the `HostAPI` client with typed interfaces for all eleven methods.
- [x] 15.4 Implement the scaffolding CLI `npx @compozy/create-extension` with template selection.
- [x] 15.5 Write four starter templates under `templates/`: `lifecycle-observer/`, `prompt-decorator/`, `review-provider/`, `skill-pack/`. Each includes a minimal working example, tests, and README.
- [x] 15.6 Write author documentation under `.compozy/docs/extensibility/` covering all the required topics.
- [x] 15.7 Write tests for the TS SDK (Vitest or Jest), the scaffolding CLI, and a smoke test that runs the lifecycle-observer template against the Go test harness from task 14.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces" for the SDK shape, `_protocol.md` sections 4-9 for the wire contract, and ADR-001/003/006 for the subprocess/transport/host-api rationale.

Package layout (for the TS package):
- `package.json`, `tsconfig.json`, `tsup.config.ts` or equivalent build config
- `src/extension.ts` — `Extension` class
- `src/transport.ts` — stdio JSON-RPC transport
- `src/host_api.ts` — `HostAPI` client
- `src/handlers.ts` — typed hook handler registration
- `src/types.ts` — TypeScript interfaces for every payload, patch, and Host API request/response
- `src/testing/mock_transport.ts` — in-memory transport for tests
- `bin/create-extension.ts` — scaffolding CLI
- `templates/lifecycle-observer/`
- `templates/prompt-decorator/`
- `templates/review-provider/`
- `templates/skill-pack/`
- `src/extension.test.ts`

Documentation layout:
- `.compozy/docs/extensibility/index.md`
- `.compozy/docs/extensibility/getting-started.md`
- `.compozy/docs/extensibility/architecture.md`
- `.compozy/docs/extensibility/hook-reference.md`
- `.compozy/docs/extensibility/host-api-reference.md`
- `.compozy/docs/extensibility/capability-reference.md`
- `.compozy/docs/extensibility/trust-and-enablement.md`
- `.compozy/docs/extensibility/testing.md`
- `.compozy/docs/extensibility/hello-world-go.md`
- `.compozy/docs/extensibility/hello-world-ts.md`

Shipping decisions:
- The TS package can live inside the main repository under a `sdk/extension-sdk-ts/` directory and be published to npm from CI, or it can live in a sibling repository. Pick whichever matches the user's existing multi-package setup. If inside the main repo, add a `Makefile` target to build and publish it.
- The scaffolding CLI is a small TypeScript binary with no external dependencies beyond `commander` or similar.
- Documentation is Markdown. If Compozy has an existing docs pipeline (mdBook, Docusaurus, etc.), hook into it; otherwise plain Markdown under `.compozy/docs/` is acceptable.

### Relevant Files
- `sdk/extension/` — From task 14. The Go SDK shape that the TS SDK mirrors.
- `_protocol.md` sections 4-9 — Wire contract.
- `_techspec.md` — Overall architecture and integration points that docs explain.
- `adrs/adr-001.md` through `adrs/adr-007.md` — Decision context referenced by the architecture doc.
- `internal/core/extension/host_api.go` — Host API method names the TS client must mirror exactly.

### Dependent Files
- External extension author repositories (future).

### Related ADRs
- [ADR-001: Subprocess-Only Extension Model](adrs/adr-001.md)
- [ADR-003: JSON-RPC 2.0 over stdio with Shared internal/core/subprocess Package](adrs/adr-003.md)
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md)

## Deliverables
- Published `@compozy/extension-sdk` TypeScript package with SDK core, `HostAPI` client, mock transport, and type definitions.
- Scaffolding CLI `npx @compozy/create-extension`.
- Four starter templates under `templates/`.
- Author documentation under `.compozy/docs/extensibility/`.
- Unit tests with 80%+ coverage for the TS SDK **(REQUIRED)**
- Integration test: run the lifecycle-observer template against the Go test harness from task 14 and verify the extension receives expected events **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Transport encodes a request and matches the response by id.
  - [ ] Transport rejects a frame larger than 10 MiB with a structured error.
  - [ ] `Extension.start()` sends `initialize` and parses the response.
  - [ ] `Extension.start()` throws on unsupported protocol version.
  - [ ] `Extension.start()` throws when accepted capabilities exceed granted capabilities.
  - [ ] `HostAPI.tasks.create` round-trips through the mock transport.
  - [ ] `HostAPI.runs.start` returns a run id from the mock host.
  - [ ] `HostAPI.memory.read` returns `exists: false` for an absent document.
  - [ ] Scaffolding CLI `create-extension lifecycle-observer my-ext` copies the template and produces a buildable project.
  - [ ] Each template's own unit tests pass locally.
- Integration tests:
  - [ ] Lifecycle-observer template, built and launched as a subprocess by the Go test harness from task 14, receives the expected `run.post_shutdown` event and exits cleanly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues (Go side)
- TypeScript build produces no errors and no warnings
- A new user can run `npx @compozy/create-extension my-ext` and have a working extension in under 5 minutes
- Documentation covers every public API and every capability
- Protocol version in the TS package matches the Go runtime version (`"1"`)

## Completion Notes

- Workspace packages shipped:
  - `sdk/extension-sdk-ts` as `@compozy/extension-sdk`
  - `sdk/create-extension` as `@compozy/create-extension`
- Author docs shipped under `.compozy/docs/extensibility/`, including getting started, architecture, hook and Host API references, capability and trust docs, testing guidance, hello-world examples, and the migration guide.
- Verification evidence:
  - `npm run build --workspace @compozy/extension-sdk --workspace @compozy/create-extension`
  - `npx vitest run sdk/extension-sdk-ts/test/*.ts sdk/create-extension/test/create-extension.test.ts`
  - `npx vitest run sdk/extension-sdk-ts/test/transport.test.ts sdk/extension-sdk-ts/test/handlers.test.ts sdk/extension-sdk-ts/test/fluent_hooks.test.ts sdk/extension-sdk-ts/test/extension.test.ts sdk/extension-sdk-ts/test/host_api.test.ts --coverage` with `83.3%` statements and `83.23%` lines for the TS SDK package
  - `go test ./sdk/extension -count=1`
  - `go test ./internal/core/extension -count=1`
  - `make verify`
