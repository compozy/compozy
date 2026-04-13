Goal (incl. success criteria):

- Implement remote extension installation plus migrate `cy-idea-factory` and the council reusable agents out of bundled core assets into a first-party extension.
- Success requires: remote GitHub archive install support in `compozy ext install`, provenance persisted and inspectable, migrated extension assets wired through setup/discovery, explicit cleanup for legacy core-managed installs, merged-scope reusable-agent verification, hardened remote `--subdir` validation, updated docs/tests, and clean `make verify`.

Constraints/Assumptions:

- Follow repository AGENTS/CLAUDE guidance and do not revert unrelated dirty files.
- The approved design is the accepted plan persisted in `.codex/plans/2026-04-13-idea-factory-remote-extension.md`.
- Remote install v1 supports only public GitHub repo archives, with `--ref` required.
- Distribution remains monorepo-hosted in this phase.
- Review identified three P2 regressions that must be fixed before completion.

Key decisions:

- `compozy ext install` supports `--remote local|github`; local remains default.
- GitHub remote v1 resolves `owner/repo` + required `--ref` + optional/expected `--subdir`.
- `cy-idea-factory` and the council roster are owned solely by the new first-party extension, not shadowed by core fallbacks.
- Installation provenance is persisted under the installed extension directory.
- Legacy bundled installs are explicitly pruned during `compozy setup` for the selected scope by removing obsolete setup-managed paths for `cy-idea-factory` and the former council agents.
- Reusable-agent verification resolves each agent with project-overrides-global precedence instead of selecting a single scope for the entire verification run.
- `--subdir` is archive-relative slash syntax only; backslashes and drive-letter forms are invalid input.

State:

- Complete.

Done:

- Read repository instructions from prompt context.
- Scanned ledger directory for cross-agent awareness and read the relevant extension setup ledger.
- Loaded required skills: `brainstorming`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, and `cy-final-verify`.
- Persisted the accepted implementation plan under `.codex/plans/2026-04-13-idea-factory-remote-extension.md`.
- Added install provenance persistence in `internal/core/extension/origin.go` and surfaced it in `compozy ext inspect`.
- Added pluggable install-source resolution for `compozy ext install`, including `--remote github`, required `--ref`, optional `--subdir`, GitHub archive download/extraction, and security checks against traversal/link abuse.
- Updated `ext install` and `ext enable` UX to print `compozy setup` follow-up hints when an extension ships skills or reusable agents.
- Migrated `cy-idea-factory` and the council roster into `extensions/cy-idea-factory/` with a declarative `extension.toml`.
- Removed `cy-idea-factory` from bundled skills and changed bundled reusable agents to an empty roster, updating embed strategy to keep `go:embed` valid with zero bundled agents.
- Fixed the three post-review regressions:
  - added explicit legacy cleanup during `compozy setup`
  - changed reusable-agent verification to merge project/global installs per agent
  - hardened `--subdir` validation against Windows-style escapes
- Added regression tests for the three fixes.
- Ran focused Go tests for affected packages.
- Ran `make verify` successfully after the fixes.

Now:

- Nothing pending in this task.

Next:

- Hand off the implementation summary with verification evidence.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-13-MEMORY-idea-factory-remote.md`
- `.codex/plans/2026-04-13-idea-factory-remote-extension.md`
- `internal/cli/extension/{install.go,install_source.go,display.go,enablement.go,root.go,display_test.go,install_source_test.go}`
- `internal/core/extension/{origin.go,origin_test.go}`
- `internal/setup/{bundle.go,bundle_test.go,catalog_effective_test.go,legacy_cleanup.go,legacy_cleanup_test.go,reusable_agent_sources.go,reusable_agents.go,reusable_agents_test_helpers_test.go,verify_test.go}`
- `internal/cli/{setup.go,setup_test.go}`
- `extensions/cy-idea-factory/`
- `agents/embed.go`
- `README.md`
- `docs/reusable-agents.md`
- `skills/compozy/{SKILL.md,references/*}`
- Commands: `go test ./internal/setup ./internal/cli ./internal/cli/extension -count=1`, `make verify`
