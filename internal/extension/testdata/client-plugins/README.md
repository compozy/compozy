# Client plugin fixtures

Runtime files copied byte-for-byte from the vendored sources:

- `.resources/open-design/plugins/open-design`: `.claude-plugin/plugin.json` and `.mcp.json`; one stdio MCP server, no packaged skills.
- `.resources/loop-engineering`: `.claude-plugin/plugin.json` and the complete `skills/` tree; seven skills, no MCP declaration.

Available source license files are retained. Repository tooling, Git metadata and unrelated media are excluded. Tests for client fields absent from these packages use separate temporary authored fixtures; never add invented components to these copies.
