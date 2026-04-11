---
status: completed
title: CLI management commands and local enablement state
type: backend
complexity: high
dependencies:
  - task_03
  - task_04
---

# Task 12: CLI management commands and local enablement state

## Overview
Add the `compozy ext` subcommand group that lets operators list, inspect, install, uninstall, enable, disable, and doctor extensions. Persist operator-local enablement state so that user and workspace extensions default to disabled and only become active once the operator explicitly enables them on this machine. This is the human-facing control surface for the feature.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Impact Analysis row for `internal/cli/extension` and ADR-005/ADR-007.
</critical>

<requirements>
- MUST register a new `ext` subcommand group on the root Cobra command with the seven subcommands listed below.
- MUST implement `compozy ext list` that enumerates every discovered extension across all three levels, showing name, version, source, enabled state, and capability list.
- MUST implement `compozy ext inspect <name>` that prints the full manifest, the declaring file path, any override records from discovery, and the active hook declarations.
- MUST implement `compozy ext install <path>` that copies an extension directory into `~/.compozy/extensions/<name>/`, prints the capability list for operator confirmation, requires `--yes` for non-interactive use, and records initial local state.
- MUST implement `compozy ext enable <name>` / `disable <name>` that toggle the operator-local enabled flag. Workspace enablement state must live outside the repo so cloning a repo does not auto-enable its extensions.
- MUST implement `compozy ext uninstall <name>` that removes an extension from `~/.compozy/extensions/`. It must refuse to touch bundled or workspace extensions.
- MUST implement `compozy ext doctor` that validates every discovered manifest, checks `min_compozy_version`, warns about priority ties on the same hook, warns about declared capabilities that have no corresponding hook or Host API usage, and reports any skill-pack or provider overlay drift (placeholder for task 13).
- MUST fit all CLI implementation into at most seven files (see Implementation Details for the grouping).
- MUST reuse the discovery and enablement store from tasks 02 and 03 — no duplicate scanning logic.
</requirements>

## Subtasks
- [x] 12.1 Create `internal/cli/extension/` package and register the `ext` subcommand group on the root command.
- [x] 12.2 Implement `list` and `inspect` commands in a shared `display.go` file.
- [x] 12.3 Implement `install` and `uninstall` commands in an `install.go` file with capability-confirmation prompt and `--yes` flag.
- [x] 12.4 Implement `enable` and `disable` commands in an `enablement.go` file that talk to the enablement store from task 02.
- [x] 12.5 Implement `doctor` in a `doctor.go` file covering manifest validation, priority tie warnings, and unused capability warnings.
- [x] 12.6 Add tests covering each command happy path plus the most important failure modes.

## Implementation Details
See TechSpec "Implementation Design → API Endpoints → CLI management surface" for the command table, "Integration Points → Trust/enablement" for the default-disabled policy, and ADR-007 for the discovery precedence rules.

File grouping to stay within the mega-task limit (seven files maximum):
- `internal/cli/extension/root.go` — `NewExtCommand(dispatcher)` that builds the `ext` parent and registers every subcommand
- `internal/cli/extension/display.go` — `list` and `inspect` implementations
- `internal/cli/extension/install.go` — `install` and `uninstall` implementations
- `internal/cli/extension/enablement.go` — `enable` and `disable` implementations
- `internal/cli/extension/doctor.go` — `doctor` implementation
- `internal/cli/extension/display_test.go` — tests for list/inspect plus install/uninstall/enable/disable
- `internal/cli/extension/doctor_test.go` — doctor-specific tests

That's 7 files. All subcommand imports happen through `root.go`.

Key invariants:
- `list` and `inspect` run without spawning any extension subprocess (pure metadata).
- `install` copies the source directory as-is; no compilation, no package manager invocation.
- `enable`/`disable` do not touch the extension directory, only the local state file.
- `doctor` is read-only; it never mutates state.
- Workspace-scoped enablement state is stored under `~/.compozy/state/workspace-extensions.json` (or similar) keyed by workspace absolute path.

### Relevant Files
- `internal/cli/root.go` — Root command registration point.
- `internal/core/extension/discovery.go` — From task 03. Used by `list`, `inspect`, `doctor`.
- `internal/core/extension/enablement.go` — From task 02. Used by `enable`, `disable`, `install`.
- `internal/core/extension/capability.go` — From task 04. Used by `doctor` to cross-reference declared vs exercised capabilities.
- `internal/cli/commands.go` — Precedent for Cobra subcommand registration patterns.

### Dependent Files
- Task 13 (declarative asset integration) reuses `enable`/`disable` state to decide which skill packs and provider overlays to activate.

### Related ADRs
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Capability confirmation on install.
- [ADR-007: Three-Level Discovery with TOML-First Manifest](adrs/adr-007.md) — Discovery precedence shown by `inspect`.

## Deliverables
- `internal/cli/extension/` package with the seven subcommands implemented.
- Updated root command registration.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests covering install → enable → list → disable → uninstall round trip **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `list` with zero extensions prints an empty table and exits zero.
  - [x] `list` with one bundled, one user, one workspace extension prints three rows with the correct source labels and enabled states.
  - [x] `inspect <name>` prints the manifest file path, parsed manifest sections, and any override records.
  - [x] `inspect <unknown>` returns a non-zero exit and a human-readable error.
  - [x] `install <path>` requires confirmation unless `--yes` is passed.
  - [x] `install <path>` with `--yes` copies the directory and records default-disabled state for user scope.
  - [x] `uninstall <bundled>` refuses with a clear message.
  - [x] `uninstall <workspace>` refuses with a clear message.
  - [x] `uninstall <user>` removes the directory and the local state entry.
  - [x] `enable <name>` marks the extension enabled in the local state.
  - [x] `disable <name>` marks the extension disabled in the local state.
  - [x] `doctor` warns when two extensions share the same `prompt.post_build` priority on the same hook.
  - [x] `doctor` warns when an extension declares `tasks.create` capability but has no hook or Host API call exercising it (detected heuristically from the manifest).
- Integration tests:
  - [x] Full round trip: `install` → `enable` → `list` shows enabled → `disable` → `list` shows disabled → `uninstall` → `list` no longer shows the extension.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- All seven `compozy ext` subcommands are documented in `compozy ext --help`
- Workspace enablement state survives git clean and git reset, proving it lives outside the repo
