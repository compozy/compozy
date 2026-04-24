Goal (incl. success criteria):

- Implement release hardening for daemon, CI, CLI/config, and docs/skills before next version.
- Success means daemon-focused tests and real smoke pass, CI lint root cause is fixed, warnings are addressed without workarounds, docs/skills match daemon-backed CLI/config, and `make verify` passes.

Constraints/Assumptions:

- Follow repository instructions in AGENTS/CLAUDE.
- No destructive git commands.
- Do not touch unrelated local edit `packages/ui/src/tokens.css`.
- User accepted removal of legacy `compozy start`.
- User accepted renaming `[start]` to `[tasks.run]` with no compatibility alias.
- Active docs/skills should be updated; historical `docs/plans/**` remains out of scope.

Key decisions:

- Fix daemon `goconst` by adding semantic constants for document/memory kinds instead of reusing `runModeTask`.
- Upgrade GitHub Actions to Node.js 24-compatible major versions rather than forcing a runner fallback.
- Keep Host API `host.runs.start` unchanged because it is an extension API verb, not the removed CLI command.

State:

- Blocked on pre-existing frontend token change that makes `make verify` fail before the Go verification stages.

Done:

- Investigated CI failure from run `24868646966`: `goconst` reported repeated `"task"` at `internal/daemon/query_service.go:853`.
- Ran prior daemon validation: race tests for daemon/API/runs passed; real daemon smoke with temp HOME and ephemeral HTTP port passed.
- Persisted accepted plan under `.codex/plans/2026-04-24-release-hardening-daemon-ci-docs.md`.
- Replaced daemon query string literals with semantic document/memory kind constants.
- Removed dead legacy `compozy start` command factory and moved task-run config from `[start]` to `[tasks.run]`.
- Updated extension `invoking_command` defaults to `tasks run` / `reviews fix` while preserving Host API `host.runs.start`.
- Upgraded CI Actions references to Node.js 24-compatible majors and set optional CI artifact upload to ignore missing files.
- Focused tests passed: `go test ./internal/core/workspace`, `go test ./internal/core/extension`, `go test ./internal/daemon -run 'Test.*(Query|Transport|RunManager|Config|Task)' -count=1`, `go test ./internal/cli -run 'Test(ApplyWorkspaceDefaults|Load|NewTasksRun|BuildConfig|TasksRun|TaskRun|Form|DaemonDocs|ActiveDocs)' -count=1`.
- Updated README, `skills/compozy/**`, and `docs/design/daemon-mockup/src/data.jsx` to daemon-backed CLI/config wording.
- Focused post-doc tests passed: `go test ./internal/cli -run 'Test(DaemonDocsUseCurrentCommandSurface|ActiveDocsAndHelpFixturesOmitLegacyArtifactRoot|ApplyWorkspaceDefaults|Form|TaskRun|TasksRun|NewTasksRun|BuildConfig)' -count=1`, `go test ./internal/core/workspace -count=1`, `go test ./internal/core/extension -count=1`, `go test ./internal/daemon -count=1`.
- `golangci-lint run --allow-parallel-runners --timeout=10m` passed with `0 issues`.
- Real daemon smoke passed with a temp binary, isolated `HOME`, ephemeral HTTP port, `[tasks.run]` config, `daemon start/status/stop`, and `workspaces resolve/list`.
- `make verify` failed in `frontend:test`: `packages/ui/tests/tokens.test.ts` still expects `font-family: "Disket Mono"`, while pre-existing local change `packages/ui/src/tokens.css` removed Disket Mono font faces and uses `JetBrains Mono`; `tokens.css` hash was unchanged by the verify run.

Now:

- Ask whether to include `packages/ui/src/tokens.css` / token test alignment in this task scope or preserve the user’s pre-existing UI edit unchanged.

Next:

- After direction, make the minimal root-cause frontend token/test fix, re-run `make verify`, then final verification skill.

Open questions (UNCONFIRMED if needed):

- Should this task include fixing the pre-existing `packages/ui/src/tokens.css` change so `make verify` can pass, or should that file remain untouched?

Working set (files/ids/commands):

- `.codex/plans/2026-04-24-release-hardening-daemon-ci-docs.md`
- `.codex/ledger/2026-04-24-MEMORY-release-hardening.md`
- `internal/daemon/query_service.go`
- `.github/workflows/*`
- `.github/actions/*`
- `internal/cli/*`
- `internal/core/workspace/config_types.go`
- `README.md`
- `skills/compozy/**`
