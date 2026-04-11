Goal (incl. success criteria):

- Fix all still-valid review issues from the provided CodeRabbit dump with root-cause changes only, add/adjust tests where behavior changes, and finish with clean `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills in use: `no-workarounds`, `systematic-debugging`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` gates completion.
- Do not touch unrelated files or use destructive git commands.
- Treat each review comment as a hypothesis; only fix it if still valid in the current tree.

Key decisions:

- Validate each issue against current code before editing.
- Prefer contained root-cause fixes over broad refactors unless the current code path already proves the architectural problem.
- Treat the `ref_id`-only `OpenURL` path as intentional behavior and simplify the redundant condition without changing semantics.
- Treat the `maybeCollectInteractiveParams` exec comment as obsolete in the current tree because `exec` already routes through `resolveExecPromptSource` instead of the interactive-form path.

State:

- Completed with clean verification.

Done:

- Read workspace instructions and required skill files.
- Scanned existing ledgers for related work and cross-agent awareness.
- Confirmed clean working tree at start.
- Inspected current implementations for `cmd/compozy/main.go`, `cmd/compozy/main_test.go`, `internal/cli/state.go`, `internal/cli/run.go`, `internal/cli/workspace_config_test.go`, `internal/core/agent/acp_convert.go`, and `internal/core/agent/tool_call_name.go`.
- Fixed `cmd/compozy/main.go` so update checking uses a caller-owned context, exposes a completion signal, and propagates notification write errors; added tests for the timeout helper, completion signal, and notification writer path.
- Fixed CLI config/prompt handling by rejecting non-positive timeouts and by ignoring stale `--prompt-file` state unless the current command invocation actually changed that flag; added regression tests.
- Renamed workspace-config subtests to `Should ...`.
- Hardened diff rendering by escaping control characters in paths and normalizing trailing newlines in rendered content; added helper assertions.
- Simplified the redundant `ref_id` open-tool branch and stopped classifying url-only inputs as web search; added focused tool-name tests.
- Ran targeted verification successfully:
  - `go test ./cmd/compozy -count=1`
  - `go test ./internal/cli -count=1`
  - `go test ./internal/core/agent -count=1`
- Ran `make verify` successfully with clean fmt/lint, `DONE 1104 tests`, and a successful build.

Now:

- Prepare the final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-07-MEMORY-review-fixes.md`
- `cmd/compozy/main.go`
- `cmd/compozy/main_test.go`
- `internal/cli/run.go`
- `internal/cli/state.go`
- `internal/cli/root_test.go`
- `internal/cli/workspace_config_test.go`
- `internal/core/agent/acp_convert.go`
- `internal/core/agent/tool_call_name.go`
- `internal/core/agent/session_helpers_test.go`
- Commands: `rg`, `sed`, `go test`, `make verify`
