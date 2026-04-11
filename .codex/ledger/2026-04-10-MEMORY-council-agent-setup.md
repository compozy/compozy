Goal (incl. success criteria):

- Implement the accepted plan so Compozy provisions bundled council reusable agents globally, exposes the reserved `compozy` MCP server to normal ACP sessions, and updates `cy-idea-factory` to run the council through real reusable subagents instead of the old inline council reference.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the accepted implementation plan from this conversation.
- Required skills in use: `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` gates completion.
- Do not touch unrelated dirty files already present in the worktree.
- `compozy setup` must keep its current `--agent` meaning for skill installation; council reusable agents are additional setup artifacts, not replacements.

Key decisions:

- `cy-idea-factory` remains the owner of the council flow; it will dispatch the six archetype reusable agents instead of delegating to a separate reusable `council` agent.
- Bundled council reusable agents are installed only to `~/.compozy/agents`.
- `compozy setup` installs those reusable agents by default during normal setup runs.
- The reserved `compozy` MCP server must be available to normal ACP sessions, not only reusable-agent-backed runs.
- Council archetype reusable agents should inherit the host runtime by omitting fixed runtime defaults.

State:

- Completed after clean `make verify`.

Done:

- Re-grounded in the current implementation of reusable agents, setup, and council skills.
- Confirmed the workspace currently has only `.claude/agents/*` archetype definitions and no bundled reusable-agent asset catalog.
- Confirmed `BuildSessionMCPServers` currently returns `nil` when no reusable agent execution context exists, which prevents ordinary skills from calling `run_agent`.
- Confirmed `cy-idea-factory` still references the older inline council reference instead of the real council skill protocol.
- Added the bundled council reusable-agent asset catalog under `agents/` with the six canonical advisor ids.
- Extended `internal/setup` and `internal/cli/setup.go` so `compozy setup` now lists, previews, installs, and reports bundled global council reusable agents under `~/.compozy/agents/`.
- Wired the reserved `compozy` MCP server into ordinary ACP session assembly through base runtime context, so non-agent top-level sessions can still call `run_agent`.
- Updated the council and `cy-idea-factory` skill references to dispatch canonical advisors through `run_agent` instead of driver-specific `.claude/agents` paths.
- Updated public docs and design docs to describe the global council provisioning and the reserved-server behavior for ordinary ACP sessions.

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-council-agent-setup.md`
- `.claude/agents/*.md`
- `skills/cy-idea-factory/SKILL.md`
- `skills/cy-idea-factory/references/council.md`
- `skills/embed.go`
- `internal/setup/*`
- `internal/core/agents/session_mcp.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/exec/exec.go`
- `internal/cli/setup.go`
- `internal/cli/skills_preflight.go`
