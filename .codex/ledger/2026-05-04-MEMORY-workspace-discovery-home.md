Goal (incl. success criteria):

- Fix issue `#139`: `workspaces register` / `resolve` must not collapse project paths to `$HOME` because of the global `~/.compozy` directory.
- Success means discovery ignores the global home-scoped `.compozy` marker during upward workspace lookup, real local workspace discovery still works, live daemon-backed relative-path commands resolve against the caller path instead of the daemon cwd, focused tests pass, and `make verify` passes.

Constraints/Assumptions:

- Follow repo instructions: no destructive git commands, use `apply_patch` for manual edits, run full verification before claiming completion.
- Required skills in use: `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, `qa-execution`, `cy-final-verify`.
- Accepted plan must be persisted under `.codex/plans/`.
- API callers using raw relative paths are ambiguous by design; the supported fix surface is the internal client/CLI boundary where the caller cwd is known.

Key decisions:

- Treat `~/.compozy` as global runtime/config state, not a workspace marker for upward discovery.
- Keep public CLI/API contracts unchanged; only path resolution behavior changes.
- Prefer best-effort canonical path comparison so symlinks do not reintroduce the bug.
- Normalize relative workspace register/resolve paths on the client side before they cross the daemon boundary so a detached daemon never resolves `.` against its own cwd.

State:

- Completed.

Done:

- Read issue `#139` and confirmed the reported symptom and reproduction.
- Traced the path flow through `internal/cli/workspace_commands.go`, `internal/api/client/operator.go`, `internal/api/core/handlers.go`, `internal/store/globaldb/registry.go`, and `internal/core/workspace/config.go`.
- Identified root cause candidate: `workspace.Discover()` walks upward on `.compozy`, which collides with the global `~/.compozy` home layout.
- Confirmed current tests cover normal nested workspace discovery and fallback-to-start behavior, but not the case where only the global home `.compozy` exists.
- Persisted the accepted implementation plan.
- Patched workspace discovery to skip the global home-scoped `.compozy` marker during upward lookup while preserving the fallback-to-start behavior.
- Added regression coverage in:
- `internal/core/workspace/config_test.go`
- `internal/store/globaldb/registry_test.go`
- `internal/cli/operator_commands_integration_test.go`
- Ran a dedicated `qa-execution` pass in `/tmp/codex-qa-issue-139`.
- Reproduced a remaining live bug the earlier tests missed: after the daemon starts from one cwd, `workspaces register .` / `resolve .` still used the daemon cwd because the client forwarded relative paths unchanged.
- Fixed the live relative-path bug in `internal/api/client/operator.go` by normalizing non-empty relative workspace paths to absolute client-side before API requests.
- Added focused regression coverage in:
- `internal/api/client/client_transport_test.go`
- `internal/cli/operator_commands_integration_test.go`
- Focused regression verification passed:
- `go test ./internal/api/client ./internal/cli -run 'TestClientNormalizesRelativeWorkspacePaths|TestWorkspaceCommandsResolveRelativePathsAgainstRealDaemon|TestWorkspaceCommandsIgnoreGlobalHomeMarkerForProjectsWithoutLocalWorkspace' -count=1`
- Rebuilt `bin/compozy` and reran a live CLI/API matrix against a fresh temp home with `COMPOZY_DAEMON_HTTP_PORT=0`.
- Live QA matrix passed for:
- explicit `workspaces register`
- `workspaces register .` from a project root without local `.compozy`
- `workspaces register .` from a nested subdir without local `.compozy`
- explicit `workspaces resolve`
- `workspaces resolve .` from inside a real local workspace
- API `POST /api/workspaces`
- API `POST /api/workspaces/resolve`
- `workspaces list` / `show`
- `sync --name complete`
- `sync`
- `archive --name complete`
- Browser smoke passed with `agent-browser`: the daemon-served workspace chooser rendered all seven canonical root paths and reported no page errors; screenshot saved to `/tmp/codex-qa-issue-139/qa/screenshots/issue139-dashboard.png`.
- Focused verification passed:
- `go test ./internal/core/workspace ./internal/store/globaldb ./internal/cli -count=1`
- First `make verify` run failed in lint on `internal/core/workspace/config.go` because `revive` flagged the intentional skip branch as an empty block.
- Refactored the conditional to preserve behavior without the empty block and re-ran the full gate.
- Final full verification passed after the QA follow-up fix: `make verify`
- Evidence:
- frontend lint/typecheck/test/build passed
- `golangci-lint run --fix --allow-parallel-runners` reported `0 issues`
- Go tests reported `DONE 3018 tests, 3 skipped in 37.866s`
- Go build completed and Playwright e2e reported `5 passed`
- final line: `All verification checks passed`
- QA artifacts written:
- `/tmp/codex-qa-issue-139/qa/verification-report.md`
- `/tmp/codex-qa-issue-139/qa/bootstrap-manifest.json`
- `/tmp/codex-qa-issue-139/qa/issues/BUG-001.md`

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-05-04-workspace-discovery-home-root.md`
- `.codex/ledger/2026-05-04-MEMORY-workspace-discovery-home.md`
- `/tmp/codex-qa-issue-139/qa/{verification-report.md,bootstrap-manifest.json,issues/BUG-001.md,screenshots/issue139-dashboard.png,outputs/*}`
- `internal/api/client/operator.go`
- `internal/api/client/client_transport_test.go`
- `internal/core/workspace/config.go`
- `internal/core/workspace/config_test.go`
- `internal/store/globaldb/registry_test.go`
- `internal/cli/operator_commands_integration_test.go`
- Commands: `go test ./internal/core/workspace ./internal/store/globaldb ./internal/cli -count=1`, `go test ./internal/api/client ./internal/cli -run 'TestClientNormalizesRelativeWorkspacePaths|TestWorkspaceCommandsResolveRelativePathsAgainstRealDaemon|TestWorkspaceCommandsIgnoreGlobalHomeMarkerForProjectsWithoutLocalWorkspace' -count=1`, live CLI/API QA matrix under `/tmp/codex-qa-issue-139/lab/sequential-live`, `agent-browser --session issue139-qa open http://127.0.0.1:59433/`, `make verify`
