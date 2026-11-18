# BUG-20261107-derived-secret-env-enable-fails: A derived secret input in a client server's env makes enable fail

- Status: verified (fixed on feat/marketplace-plugin-catalog)
- Fixed in: `68d106e25`
- Impact (user-side): Blocking for the affected plugins
- Severity: High; Priority: P1
- Persona Affected: Bruno
- Journey Step: Add a client plugin whose root `.mcp.json` passes a secret through `env`
- Scenarios: ET-agent-plugin-source-install, ET-agent-plugin-marketplace-install
- Origin: spec marketplace-plugin-catalog S8 integrated walk (isolated lab, 2026-11-07)

## Reproduction

Cursor-layout plugin with no `mcpServers` and a root `.mcp.json`:

```json
{ "mcpServers": { "demo-local": {
  "command": "${CLAUDE_PLUGIN_ROOT}/bin/demo-mcp",
  "args": ["--host", "${DEMO_HOST}", "--mode", "${DEMO_MODE:-read}"],
  "env": { "DEMO_API_KEY": "${DEMO_API_KEY}" } } } }
```

`compozy extension install <dir> --allow-unverified --yes --enable --input DEMO_API_KEY=x --input
DEMO_HOST=y` installs, then reports `Couldn't turn on: extension_enable_failed`.
`compozy extension enable demo-api` → `extension.mcp_server.env.DEMO_API_KEY must move secret-like
values to secret_env`. The same package with a non-secret name in `env` (`DEMO_REGION`) turns on and is
healthy. This is the `_dx.md` golden example pattern (`"NOTION_TOKEN": "${NOTION_TOKEN}"`).

Expected: a derived secret referenced from `env` is mapped to `secret_env` (vault ref) so the
server starts.

## Re-walk 2026-11-07 (integration 30fcdfee8, fresh isolated lab)

The same `.mcp.json` fixture (`env: {"DEMO_API_KEY": "${DEMO_API_KEY}"}`) installed with
`--enable --input DEMO_API_KEY=… --input DEMO_HOST=…` reports Enabled true, State active, Health healthy.
The secret value does not appear in status output or the daemon log.
