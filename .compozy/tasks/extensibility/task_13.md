---
status: completed
title: Declarative asset integration for skill packs and provider overlays
type: backend
complexity: high
dependencies:
  - task_03
  - task_12
---

# Task 13: Declarative asset integration for skill packs and provider overlays

## Overview
Wire the declarative assets discovered in task 03 into the runtime. Extension-provided skill packs must be materialized into agent skill directories through the existing `internal/setup` and `skills_preflight` paths alongside bundled Compozy skills. Extension-provided provider overlays (`[[providers.ide]]`, `[[providers.review]]`, `[[providers.model]]`) must layer onto the base registries in `internal/core/provider` and `internal/core/agent/registry_launch` so commands like `compozy fetch-reviews` and ACP runtime resolution can pick them up without recompiling Compozy.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Impact Analysis rows for `internal/setup` + `skills_preflight` and `internal/core/provider` + `internal/core/agent`.
</critical>

<requirements>
- MUST extend `internal/setup` so extension skill packs (from enabled extensions only) are materialized into agent-visible skill directories using the same install/verify abstractions as bundled skills.
- MUST extend `internal/cli/skills_preflight.go` so the preflight drift check covers both bundled skills and enabled extension skill packs for the selected agent.
- MUST implement an overlay layer for the `internal/core/provider` registry that stacks extension-declared providers on top of the built-in catalog without mutating the base catalog globally.
- MUST implement an equivalent overlay for the `internal/core/agent/registry_launch` ACP runtime entries so extensions can register new IDE adapters declaratively.
- MUST make overlay assembly happen during command bootstrap (before plan/execute) so declarative assets are available even for commands that do not spawn executable extensions.
- MUST respect operator-local enablement from task 12: disabled extensions contribute neither skill packs nor provider overlays.
- MUST surface drift and override information in `compozy ext doctor` from task 12 (add the missing pieces left as placeholders there).
- MUST NOT mutate the built-in provider registry or the bundled `skills.FS` — all additions happen through the overlay layer.
</requirements>

## Subtasks
- [x] 13.1 Extend `internal/setup` with functions that materialize extension skill packs (`InstallExtensionSkillPacks`, `VerifyExtensionSkillPacks`) sharing the core install/verify abstractions.
- [x] 13.2 Extend `internal/cli/skills_preflight.go` so drift detection covers bundled plus extension-provided skills and handles the combined install flow.
- [x] 13.3 Introduce an overlay registry for `internal/core/provider` that wraps the base registry with extension-declared entries for the command's lifetime.
- [x] 13.4 Introduce an equivalent overlay for `internal/core/agent/registry_launch` entries so declarative ACP adapters work.
- [x] 13.5 Hook the overlay assembly into command bootstrap so every relevant command consumes the same overlay snapshot.
- [x] 13.6 Extend `compozy ext doctor` to report skill-pack drift and provider overlay conflicts.
- [x] 13.7 Add tests that cover skill-pack install/verify, provider overlay resolution, and doctor drift reports.

## Implementation Details
See TechSpec "Data Flow → Provider resolution (non-run and pre-run commands)" for the overlay lifecycle and "Integration Points → Existing Compozy components touched" for the affected files.

Files touched:
- `internal/setup/extensions.go` — new file with `InstallExtensionSkillPacks`, `VerifyExtensionSkillPacks`
- `internal/cli/skills_preflight.go` — modified preflight flow
- `internal/core/provider/overlay.go` — new file with `OverlayRegistry`
- `internal/core/agent/registry_overlay.go` — new file adding ACP runtime overlays
- `internal/cli/extension/doctor.go` — extend doctor (from task 12) with drift checks
- `internal/setup/extensions_test.go`
- `internal/core/provider/overlay_test.go`

Key invariants:
- The base `skills.FS` and the base provider registry are never mutated. All additions go through the overlay.
- The same discovered declarative-asset inventory drives both skill pack installation and provider overlay assembly; task 03's `DeclaredSkillPacks` and `DeclaredProviders` are the single source of truth.
- Disabled extensions are excluded before the overlay is assembled. They do not contribute any asset even temporarily.
- Overlay state is per-command (scoped to a single `compozy` invocation); nothing persists across invocations.

### Relevant Files
- `internal/setup/bundle.go` — Existing `ListBundledSkills`, `InstallBundledSkills`, `VerifyBundledSkills`. Pattern to mirror.
- `internal/cli/skills_preflight.go` — Existing preflight drift flow.
- `internal/core/provider/registry.go` — Base provider registry.
- `internal/core/agent/registry_launch.go` — Base ACP runtime registry.
- `internal/core/extension/discovery.go` — From task 03. Source of `DeclaredSkillPacks` and `DeclaredProviders`.
- `internal/core/extension/enablement.go` — From task 02. Used to filter disabled extensions.
- `internal/cli/extension/doctor.go` — From task 12. Extended with drift checks here.

### Dependent Files
- `compozy fetch-reviews` flow consumes the provider overlay to resolve extension-declared review providers.
- `compozy start` and `compozy fix-reviews` consume the ACP runtime overlay when picking agents.
- Task 14 (Go SDK) may expose helpers for declaring providers from extension code.

### Related ADRs
- [ADR-007: Three-Level Discovery with TOML-First Manifest](adrs/adr-007.md) — Discovery and enablement integration.
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Declarative assets are gated by the `providers.register` and `skills.ship` capabilities.

## Deliverables
- Skill pack install/verify integration in `internal/setup` and `skills_preflight`.
- Provider and ACP runtime overlay registries with command-scoped activation.
- Extended `compozy ext doctor` drift reporting.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests verifying that a declarative extension contributes skills and providers without spawning a subprocess **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `InstallExtensionSkillPacks` copies markdown files from an enabled extension's declared skill paths into the target agent skill directory.
  - [x] `VerifyExtensionSkillPacks` reports drift when a file exists on disk but has different content than the manifest-declared source.
  - [x] Disabled extensions contribute zero skill files.
  - [x] Provider `OverlayRegistry.Get(name)` returns the overlay entry when present and falls back to the base registry when absent.
  - [x] Base provider registry is not mutated when overlays are added.
  - [x] ACP runtime overlay resolves an extension-declared IDE adapter for `compozy start --ide <ext-adapter>`.
  - [x] `doctor` reports a warning when two enabled extensions declare the same review provider name.
- Integration tests:
  - [x] A workspace extension that declares only a skill pack (no subprocess) is discovered, marked enabled, and its skills appear in the agent's skill directory after `skills_preflight` runs.
  - [x] A workspace extension that declares only a review provider (no subprocess) is discovered, marked enabled, and `compozy fetch-reviews --provider <ext-provider>` resolves to it.
  - [x] A disabled extension with a declared skill pack contributes no files to the agent's skill directory.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- A pure-declarative extension (no subprocess) can ship skills and providers and be consumed by commands that never spawn extensions.
- `compozy ext doctor` reports both skill-pack drift and provider overlay conflicts clearly.
