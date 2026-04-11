# Task Memory: task_13.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Wire enabled extension skill packs and declarative provider overlays into runtime bootstrap, setup/preflight, and `compozy ext doctor` for task 13.
- Keep scope to setup/preflight, provider/ACP overlay assembly, doctor drift/conflict reporting, and the required tests.

## Important Decisions
- Use one command-scoped discovery/bootstrap snapshot so the same enabled-extension inventory drives both skill-pack handling and provider/agent overlays.
- Reuse existing setup path/install/verify helpers for extension skill packs instead of creating a second installation model.
- Treat declarative review providers as overlay aliases that resolve through `provider.ResolveRegistry(...)`, while declarative IDE entries map metadata into command-scoped `agent.Spec` overlays without mutating the built-in catalog.
- Combined skills preflight blocks only on missing bundled Compozy skills; missing extension skill packs are refreshable changes because declarative extensions may need first-time materialization into the agent skill directory.

## Learnings
- `agent.SetupAgentName(...)` is the bridge between runtime IDE selection and setup/preflight agent directories, including declarative IDE overlays that specify `metadata.agent_name`.
- `requiredSkillState.Scope()` needed to treat empty scopes the same as `unknown`; otherwise a zero-value bundled result could incorrectly shadow extension verification scope.
- Focused coverage for the task-13 files clears the threshold after adding helper-path tests: `internal/setup/extensions.go` 82.6%, `internal/core/provider/overlay.go` 86.8%, `internal/core/agent/registry_overlay.go` 83.7%, `internal/cli/extensions_bootstrap.go` 84.8%, `internal/cli/skills_preflight.go` 80.0%, `internal/cli/extension/doctor.go` 84.7%.

## Files / Surfaces
- `internal/setup`
- `internal/setup/extensions.go`
- `internal/setup/extensions_test.go`
- `internal/cli/extensions_bootstrap.go`
- `internal/cli/extensions_bootstrap_test.go`
- `internal/cli/skills_preflight.go`
- `internal/cli/skills_preflight_test.go`
- `internal/cli/run.go`
- `internal/cli/extension/doctor.go`
- `internal/cli/extension/doctor_test.go`
- `internal/core/provider`
- `internal/core/provider/overlay.go`
- `internal/core/provider/overlay_test.go`
- `internal/core/agent`
- `internal/core/agent/registry_overlay.go`
- `internal/core/agent/registry_overlay_test.go`
- `internal/core/fetch.go`
- `internal/core/run/executor/review_hooks.go`
- `internal/core/extension/assets.go`

## Errors / Corrections
- A stale root test still expected the old bundled-only refresh message; updated it to the new required-skills wording.
- One coverage-only ACP client test flaked once during a broad coverage sweep, but reran cleanly and did not reproduce under `make verify`.
- Lint required replacing several `range` value copies in the new overlay/setup code and removing one ineffective assignment in a provider test stub.

## Ready for Next Run
- Task implementation and verification are complete; only downstream tasks that build on declarative overlays or SDK exposure should continue from here.
