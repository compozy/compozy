# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Publish user-facing reusable-agent docs that match the implemented CLI and validation behavior.
- Commit copyable agent fixtures and bind tests to those exact files so docs and examples drift together.

## Important Decisions
- Put the focused guide at `docs/reusable-agents.md` and keep the top-level introduction plus command surface in `README.md`.
- Commit example fixtures under `docs/examples/agents/` instead of `.compozy/agents/` so the repository does not ship live reusable agents in its own workspace config.
- Use the minimal `reviewer` fixture for the documented `agents inspect` and `exec --agent` examples because its output is stable and does not require MCP environment variables.
- Use `repo-copilot` as the documented MCP example so the docs can show placeholder expansion and multi-server summaries with real validation coverage.

## Learnings
- `compozy agents inspect` always prints the full report and only then returns exit code 1 for invalid agents; docs should describe that behavior explicitly.
- `compozy agents list` reports valid agents first and invalid definitions afterward, so malformed directories do not hide good agents from discovery.
- The most stable inspect example lines are `Agent`, `Status`, `Source`, `Title`, `Description`, `Runtime defaults`, `MCP servers`, and `Validation`; absolute path lines are environment-specific and should stay out of shortened docs snippets.

## Files / Surfaces
- `README.md`
- `docs/reusable-agents.md`
- `docs/examples/agents/reviewer/AGENT.md`
- `docs/examples/agents/repo-copilot/AGENT.md`
- `docs/examples/agents/repo-copilot/mcp.json`
- `internal/core/agents/doc_examples_test.go`
- `internal/cli/reusable_agents_doc_examples_test.go`

## Errors / Corrections
- Initial README command docs were missing the reusable-agent surface and several current `exec` flags (`--agent`, `--verbose`, `--tui`, `--persist`, `--run-id`, `raw-json`). The docs were corrected to match the implemented CLI instead of the older text.

## Ready for Next Run
- `make verify` passed after removing `t.Parallel()` from the new CLI doc-example tests; those tests touch process-global state through working-directory changes and the shared ACP client test hook.
- Tracking files have been updated to completed. If a follow-up edit is needed, keep task/memory artifacts out of the code commit unless the workflow explicitly requires them.
- Code/docs/test changes were committed locally as `60a179c` (`docs: add reusable agent guide and fixtures`).
