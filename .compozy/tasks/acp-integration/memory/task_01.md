# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Deliver the ACP foundation for task_01: content model, registry, client/session wrapper, mock ACP server helper, and tests.
- Keep the branch buildable before task_02 by preserving any compatibility surface still required by the current runtime.
- Result: completed with clean verification and package coverage above the task threshold.

## Important Decisions
- Treat the PRD + techspec as the approved design baseline; do not reopen design scope during implementation.
- Use real `acp-go-sdk` docs/API before writing wrapper code instead of guessing transport/session shapes.
- Keep `agent.Command(...)` as a legacy compatibility shim in `internal/core/agent/ide.go` until task_02 migrates runtime callers.
- Export the new registry type as `agent.Spec` instead of `AgentSpec` because `revive` rejects the stuttering name under this repo's zero-lint policy.
- Use the production default `3s` shutdown timeout in the ACP integration test helper so `-race` validates lifecycle behavior instead of timing out the helper subprocess.

## Learnings
- The prompt's lowercase `/dev/...` paths map to this checkout under `/Users/pedronauck/Dev/compozy/looper`.
- `internal/core/run` still calls `agent.Command(...)`, so task_01 cannot blindly remove compatibility with the legacy runtime before task_02 lands.
- `internal/setup/agents.go` is unrelated to the ACP agent registry and should not be conflated with `internal/core/agent`.
- `github.com/coder/acp-go-sdk@v0.6.3` provides the client/session stdio transport surface needed for ACP, but it does not expose a Compozy-ready usage model, so `model.Usage` remains repo-owned.
- Repo-wide tests were stable once the ACP permission-outcome assertion stopped depending on a direct SDK union field selector and the helper shutdown timeout matched the production client default.

## Files / Surfaces
- `internal/core/agent/client.go`
- `internal/core/agent/client_test.go`
- `internal/core/agent/ide.go`
- `internal/core/agent/legacy_command_test.go`
- `internal/core/agent/registry.go`
- `internal/core/agent/registry_test.go`
- `internal/core/agent/session.go`
- `internal/core/agent/session_helpers_test.go`
- `internal/core/model/content.go`
- `internal/core/model/content_test.go`
- `internal/core/model/model.go`
- `internal/core/model/model_test.go`
- `internal/core/api.go`
- `internal/core/run/types.go`
- `go.mod`
- `go.sum`
- `.compozy/tasks/acp-integration/_techspec.md`
- `.compozy/tasks/acp-integration/adrs/adr-001.md`
- `.compozy/tasks/acp-integration/adrs/adr-002.md`
- `.compozy/tasks/acp-integration/adrs/adr-003.md`
- `.compozy/tasks/acp-integration/adrs/adr-004.md`
- `.compozy/tasks/acp-integration/adrs/adr-005.md`

## Errors / Corrections
- Initial attempt to read `cy-workflow-memory` from `~/.agents/...` failed; the installed skill for this repo lives under `.agents/skills/cy-workflow-memory/SKILL.md`.
- ACP close tests initially failed under `-race` because the helper forced a `1s` shutdown timeout; aligning it with the production `3s` default removed the false failure.
- Repo-wide `gotestsum` runs intermittently surfaced the ACP permission outcome union via the wrong selector spelling; the test now validates the cancelled variant without depending on that direct selector.

## Ready for Next Run
- Task_01 is complete. Task_02 should wire `internal/core/run` onto `agent.Client`, `agent.Session`, and `model.SessionUpdate`, then remove the remaining legacy command path.
