Goal (incl. success criteria):

- Implement `resources.agents` plus setup integration for extension-shipped skills and reusable agents without removing mandatory core workflow skills.
- Success requires working discovery/validation/install/verify flows, updated CLI/setup behavior, tests, and clean `make verify`.

Constraints/Assumptions:

- Follow repository AGENTS/CLAUDE guidance and do not touch unrelated dirty files.
- Core workflow skills remain bundled in this change.
- Extension assets only influence setup when the owning extension is enabled.
- Reusable agents should respect the scope chosen in `compozy setup` (`project` vs `global`).

Key decisions:

- Keep core assets winning name conflicts over extension assets for this phase.
- Extend manifest resources with `agents` guarded by a new `agents.ship` capability.
- Unify setup catalogs rather than adding a parallel extension-only setup command.
- Keep reusable-agent installation behavior automatic for eligible extension assets, but make the install root follow the chosen setup scope.

State:

- In progress.

Done:

- Read repository instructions from prompt context.
- Scanned ledger directory for cross-agent awareness and read relevant extension ledgers.
- Loaded required skills: `brainstorming`, `golang-pro`, `testing-anti-patterns`.
- Reconfirmed current setup, extension discovery, and validation entrypoints.
- Added `resources.agents` / `agents.ship` to manifest structures and validation, including raw manifest decode support.
- Extended extension discovery/assets extraction with reusable-agent declarations and setup metadata.
- Added setup-side effective catalog resolution with core-wins conflict handling plus extension precedence.
- Added generic selected-skill install/preview plumbing so `setup` can install mixed core + extension skills.
- Wired `compozy setup` to load enabled extension assets, list sources, warn on ignored conflicts, and install extension reusable agents automatically.
- Wired workflow preflight to verify only effective extension skill packs.
- Extended `ext doctor` with `agents.ship` capability evidence, effective setup conflict warnings, and reusable-agent drift checks.
- Added regression tests across manifest validation, asset extraction, discovery, setup catalog/install, CLI setup list, and doctor warnings.
- Refactored reusable-agent preview/install/verify APIs to be scope-aware with explicit configs and effective-scope detection.
- Wired `compozy setup` to pass the chosen `project`/`global` scope through reusable-agent preview/install flows and updated CLI copy/output accordingly.
- Updated reusable-agent verification and `ext doctor` warnings to report the effective scope instead of assuming global-only.
- Added regression coverage for project/global reusable-agent install paths, scope precedence, scope hints, project drift, and CLI project-scope propagation.
- Ran targeted tests: `go test ./internal/setup` and `go test ./internal/cli/...` both passed.

Now:

- Run the full verification pipeline and confirm the final state.

Next:

- If `make verify` passes, report the scoped reusable-agent behavior change with verification evidence.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-13-MEMORY-extension-setup-assets.md`
- `internal/core/extension/{manifest.go,manifest_validate.go,assets.go,discovery.go}`
- `internal/core/extension/{manifest_load.go,assets_test.go,discovery_test.go,manifest_test.go}`
- `internal/setup/{types.go,catalog.go,bundle.go,reusable_agents.go,extensions.go,skills_selected.go,catalog_effective.go,catalog_helpers.go,reusable_agent_sources.go}`
- `internal/cli/{setup.go,setup_assets.go,skills_preflight.go,run.go}`
- `internal/cli/extension/doctor.go`
- Commands: `rg`, `sed`, `nl`, `gofmt -w ...`, `go test ./internal/setup`, `go test ./internal/cli/...`, `go test ./internal/core/extension -run ...`, `make verify`
