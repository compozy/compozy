---
status: completed
title: Scaffold internal/core/extension package with manifest parser and enablement model
type: backend
complexity: medium
dependencies:
  - task_01
---

# Task 02: Scaffold internal/core/extension package with manifest parser and enablement model

## Overview
Create the new `internal/core/extension` package and implement the manifest parser (TOML primary, JSON fallback), the shared types used across subsequent extension tasks, and the operator-local enablement model that records which user and workspace extensions have been explicitly enabled on this machine. This task does not wire anything into the runtime; it only produces a correct, tested foundation for the rest of the feature.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Technical requirements derive from `_techspec.md` and ADR-005/ADR-007.
</critical>

<requirements>
- MUST create `internal/core/extension` as a new Go package with no runtime side effects at import time.
- MUST define the `Manifest`, `ExtensionInfo`, `SubprocessConfig`, `SecurityConfig`, `HookDeclaration`, `ResourcesConfig`, `ProvidersConfig`, and `ProviderEntry` Go types in line with the TechSpec "Core Interfaces" and "Manifest schema" sections.
- MUST implement a loader that tries `<extension-dir>/extension.toml` first and falls back to `<extension-dir>/extension.json` exactly as specified in ADR-007.
- MUST validate each loaded manifest: required fields, capability names against the taxonomy from ADR-005, hook event names against the canonical list in `_protocol.md` section 6.5, and `HookDeclaration.Priority` within `[0, 1000]`.
- MUST enforce `min_compozy_version` at parse time using a semver comparison against `internal/version.Version`.
- MUST implement a local enablement store that persists per-extension enabled state outside the repository (user scope: `~/.compozy/extensions/<name>/.compozy-state.json`; workspace scope: in a local state file under the user's home so cloning a repo does not auto-enable its extensions).
- MUST default bundled extensions to enabled and user/workspace extensions to disabled until the operator explicitly enables them.
- MUST NOT spawn any subprocess or touch the event bus in this task.
- MUST keep all functions context-aware (`context.Context` first) per project conventions.
</requirements>

## Subtasks
- [x] 02.1 Create the `internal/core/extension` package directory with a `doc.go` describing the package's purpose.
- [x] 02.2 Implement the `Manifest` and related types in a dedicated `manifest.go` file with TOML and JSON struct tags.
- [x] 02.3 Implement the manifest loader (`LoadManifest(ctx, dir)`) with TOML-first/JSON-fallback semantics and structured errors.
- [x] 02.4 Implement manifest validation covering required fields, capability taxonomy, hook event taxonomy, priority range, and `min_compozy_version` checks.
- [x] 02.5 Implement the operator-local enablement store: loader, saver, default policy per source, and explicit enable/disable mutators.
- [x] 02.6 Write table-driven unit tests for parser (valid/invalid TOML and JSON) and enablement store (default, enable, disable, persistence round-trip).

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces" for the `Manifest` shape and "Integration Points" for how the enablement store relates to CLI commands added later. See ADR-005 for the capability taxonomy and ADR-007 for the discovery and manifest rules.

Place files under:
- `internal/core/extension/doc.go`
- `internal/core/extension/manifest.go`
- `internal/core/extension/manifest_load.go`
- `internal/core/extension/manifest_validate.go`
- `internal/core/extension/enablement.go`
- `internal/core/extension/manifest_test.go`
- `internal/core/extension/enablement_test.go`

The capability taxonomy must be represented as a typed set so later tasks (capability enforcement, CLI) can validate against it without string comparisons scattered across the codebase.

### Relevant Files
- `internal/version/version.go` — Source of `Version` used for `min_compozy_version` comparison.
- `_techspec.md` — Manifest schema and enablement semantics.
- `_protocol.md` section 6.5 — Canonical hook event taxonomy for validation.
- `adrs/adr-005.md` — Capability taxonomy.
- `adrs/adr-007.md` — Discovery and manifest rules.

### Dependent Files
- Future task 03 (discovery) consumes `LoadManifest` and `Manifest`.
- Future task 04 (capability enforcement) consumes the capability taxonomy defined here.
- Future task 05 (dispatcher) consumes `HookDeclaration`.
- Future task 12 (CLI) consumes the enablement store to power `compozy ext enable/disable`.

### Related ADRs
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Capability taxonomy defined here.
- [ADR-007: Three-Level Discovery with TOML-First Manifest](adrs/adr-007.md) — Manifest format and precedence rules.

## Deliverables
- New package `internal/core/extension` with scaffold, types, manifest loader, validator, and enablement store.
- Documented capability taxonomy and hook event taxonomy constants.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for enablement store persistence round-trip **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Loader returns a parsed manifest when only `extension.toml` is present.
  - [x] Loader returns a parsed manifest when only `extension.json` is present.
  - [x] Loader prefers `extension.toml` when both files exist and logs a warning about the ignored JSON file.
  - [x] Loader returns a structured error when neither file is present.
  - [x] Validator rejects an unknown capability name with a message naming the offending capability.
  - [x] Validator rejects an unknown hook event name with a message naming the offending event.
  - [x] Validator rejects `HookDeclaration.Priority` outside `[0, 1000]`.
  - [x] Validator rejects a manifest whose `min_compozy_version` is newer than the current `version.Version`.
  - [x] Enablement store defaults bundled source to `enabled = true`, user/workspace sources to `enabled = false`.
  - [x] Enablement store persists explicit enable/disable across process boundaries (round-trip via `t.TempDir()`).
- Integration tests:
  - [x] Loader plus validator produces a usable `Manifest` from a realistic fixture with subprocess, security, hooks, resources, and providers sections.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- `internal/core/extension` package compiles cleanly and exports the types used by downstream tasks
- Capability taxonomy and hook event taxonomy are single sources of truth referenced by subsequent tasks
