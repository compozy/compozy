Goal (incl. success criteria):

- Identify the current daemon/API architecture in the looper repository and the most natural integration points for a bundled web UI that starts with the daemon by default.
- Success means: concise read-only architecture report with current package map, candidate embedding/serving locations, constraints/risks, and concrete file references.

Constraints/Assumptions:

- Read-only analysis only; do not modify production code.
- Follow repository guidance from `AGENTS.md` and `CLAUDE.md`.
- Do not use web search for local code discovery.
- Read-only awareness of other `.codex/ledger/*-MEMORY-*.md` files is required.

Key decisions:

- Treat `internal/daemon` + `internal/api/{core,httpapi,udsapi,client}` as the active daemon surface.
- Use `internal/config/home.go` and `internal/cli/daemon.go` as the operational bootstrap boundary.
- Treat `docs/design/daemon-mockup` as prototype-only, not a runtime asset pipeline.

State:

- Context gathered from repository files and existing daemon/design ledgers.
- Final report pending.

Done:

- Read daemon, transport, home-layout, CLI bootstrap, and embed-related files.
- Confirmed there is no current static-file serving path in the daemon transport stack.
- Confirmed existing embed conventions live in `agents`, `skills`, `internal/core/prompt`, and `internal/core/extension`.

Now:

- Produce the concise architecture report requested by the user.

Next:

- If needed, refine the report with additional file-line references, but do not change code.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `internal/daemon/boot.go`
- `internal/daemon/host.go`
- `internal/api/httpapi/server.go`
- `internal/api/udsapi/server.go`
- `internal/api/core/routes.go`
- `internal/api/core/interfaces.go`
- `internal/api/client/client.go`
- `internal/cli/daemon.go`
- `internal/config/home.go`
- `agents/embed.go`
- `skills/embed.go`
- `internal/core/prompt/templates.go`
- `internal/core/extension/discovery_bundled.go`
- `docs/design/daemon-mockup/Compozy Daemon Workspace.html`
- `docs/plans/2026-04-17-compozy-daemon-design.md`
