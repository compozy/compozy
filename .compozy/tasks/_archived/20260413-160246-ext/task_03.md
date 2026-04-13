---
status: completed
title: Three-level discovery pipeline with provider and skill asset extraction
type: backend
complexity: medium
dependencies:
  - task_02
---

# Task 03: Three-level discovery pipeline with provider and skill asset extraction

## Overview
Implement the extension discovery pipeline that enumerates extensions across three levels — bundled (via `go:embed`), user (`~/.compozy/extensions/`), and workspace (`<workspace-root>/.compozy/extensions/`) — resolves precedence (workspace wins over user wins over bundled), and extracts the declarative assets each manifest ships: provider registrations under `[[providers.*]]` and skill packs under `[resources.skills]`. This task does not execute any extension code; it only loads manifests and indexes declarative assets so later tasks can wire them into the runtime.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` and ADR-007.
</critical>

<requirements>
- MUST implement `Discovery.Discover(ctx)` that returns all enabled manifests across the three levels in a deterministic order.
- MUST apply workspace > user > bundled precedence when two levels declare an extension with the same name, recording every override in a structured audit entry for `compozy ext inspect`.
- MUST load bundled extensions through a `go:embed` filesystem rooted at `internal/core/extension/builtin/`.
- MUST resolve operator-local enablement via the enablement store from task 02. Disabled extensions are discovered (listed) but not returned to downstream runtime consumers.
- MUST extract provider and skill-pack declarations from each manifest into a typed inventory structure consumable by tasks 12 (CLI), 13 (declarative integration), and future command bootstraps.
- MUST NOT spawn any subprocess.
- MUST NOT depend on the extension manager or host API.
- MUST keep the bundled `go:embed` root empty in v1 (no first-party extensions ship yet) but the plumbing must be in place.
</requirements>

## Subtasks
- [x] 03.1 Create the bundled extension filesystem rooted at `internal/core/extension/builtin/` with a stub `go:embed` directive.
- [x] 03.2 Implement the discovery scanner that walks bundled, user, and workspace directories, loads each manifest via task 02's loader, and logs structured errors for malformed manifests without aborting the scan.
- [x] 03.3 Implement precedence resolution producing one effective manifest per extension name, plus an override record listing which level won and which lost.
- [x] 03.4 Implement the provider and skill-pack inventory extractor that produces typed `DeclaredProviders` and `DeclaredSkillPacks` values for downstream consumers.
- [x] 03.5 Surface the enablement filter so discovery callers can ask for "all discovered" (CLI listing) or "only enabled" (runtime startup).
- [x] 03.6 Write table-driven unit tests covering happy path, precedence conflict, malformed manifest handling, and provider/skill inventory extraction.

## Implementation Details
See TechSpec "System Architecture → Data Flow → Run startup" steps 3 and 4 for the discovery ordering, "Integration Points → Existing Compozy components touched" for how the declared assets feed the provider overlays and skill preflight, and ADR-007 for the precedence rules and manifest format fallback.

Place files under:
- `internal/core/extension/discovery.go` — scanner + precedence + enablement filter
- `internal/core/extension/discovery_bundled.go` — `go:embed` root loader
- `internal/core/extension/assets.go` — `DeclaredProviders`, `DeclaredSkillPacks`, extraction helpers
- `internal/core/extension/builtin/doc.go` — placeholder to anchor the embed directive
- `internal/core/extension/discovery_test.go`
- `internal/core/extension/assets_test.go`

Bundled extensions in v1 are empty. Document this in the package doc so reviewers understand why the `go:embed` root has no actual extensions.

### Relevant Files
- `internal/core/extension/manifest.go` — From task 02. Loader is reused here.
- `internal/core/extension/enablement.go` — From task 02. Used to filter discovered manifests.
- `skills/embed.go` — Precedent for `go:embed` directive usage in this codebase.
- `_techspec.md` → Data Flow section — Discovery ordering requirements.
- `adrs/adr-007.md` — Precedence and fallback rules.

### Dependent Files
- Task 07 (run-scope bootstrap) calls discovery during command bootstrap.
- Task 12 (CLI management) calls discovery for `compozy ext list` / `inspect`.
- Task 13 (declarative asset integration) consumes `DeclaredProviders` and `DeclaredSkillPacks`.

### Related ADRs
- [ADR-007: Three-Level Discovery with TOML-First Manifest](adrs/adr-007.md) — Governs this task end to end.

## Deliverables
- Discovery scanner with three-level walk, precedence resolution, and enablement filter.
- Typed asset inventory for providers and skill packs.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests exercising a realistic three-level fixture **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Discovery returns an empty slice when no extensions are installed at any level.
  - [x] Discovery returns the bundled extension when only bundled is populated.
  - [x] Discovery returns the user extension when the same name exists in bundled and user, with an override record pointing at the bundled loser.
  - [x] Discovery returns the workspace extension when the same name exists in all three levels, with override records for both losers.
  - [x] A malformed manifest at the workspace level does not prevent bundled and user extensions from being discovered; the error is logged and reported in a per-level failure list.
  - [x] Enablement filter returns bundled extensions by default and hides disabled user/workspace extensions.
  - [x] Asset extractor returns `DeclaredProviders` grouped by category (ide, review, model).
  - [x] Asset extractor returns `DeclaredSkillPacks` with the absolute paths each pack resolves to.
- Integration tests:
  - [x] End-to-end discovery over a `t.TempDir()` fixture containing one bundled stub plus one user extension plus one workspace extension yields the expected three entries, override records, and typed asset inventory.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- Discovery is consumable by CLI and runtime bootstrap without additional glue
- Override records are explicit and structured so `compozy ext inspect` can explain what won and why
