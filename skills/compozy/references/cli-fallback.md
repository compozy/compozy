# CompozyOS CLI fallback

Native tool transport is unavailable in this runtime. Use the following public
CLI entry points only within the authorization already granted to this session.
Preserve the inherited daemon connection, profile, and workspace; do not switch
homes, credentials, or identities to bypass a denial.

- Read runtime health: `compozy status -o json` or `compozy doctor --help`.
- Read this session: `compozy session status <session-id> -o json`.
- Inspect retained work: `compozy session history <session-id> --help` and
  `compozy session events <session-id> --help`, then select bounded reads.
- Inspect available command-palette operations: `compozy cmd-palette list --help`
  and `compozy cmd-palette inspect --help`.
- Find exact syntax before any other authorized operation with `compozy --help`
  and `compozy <command> --help`. Use structured output and confirm mutations
  through a subsequent read.

HTTP/UDS clients may use their already-authorized connection for the corresponding
session reads at `GET /api/sessions/{session_id}` and
`GET /api/workspaces/{workspace_id}/sessions/{session_id}/events`; preserve scope and pagination.
Do not invent routes or weaken access controls if a surface is unavailable.

The native skill reader is unavailable. Do not use operator-only skill commands
or filesystem reads to bypass that boundary. Agent-to-agent messages require the
native session messaging capability; do not impersonate an operator with the CLI.
Provider background jobs cannot wake this session. Do not promise a later reply
without a confirmed daemon wake; report the unavailable capability when necessary.
