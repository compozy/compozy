---
id: ET-cli-mcp-runtime-verbs
area: ET
title: MCP list, allow, import and session MCP CLI verbs match their documented shapes
persona: Ada
journey: J-operate-compozy-from-mcp-client
expected: `compozy mcp list`, `compozy mcp allow`, `compozy mcp import` and `compozy session mcp` run against the lab daemon with human, `-o json` and `-o toon` output; JSON shapes, error codes and exit codes match `_dx.md` (as shipped), and the native tools `compozy__mcp_list`, `compozy__session_mcp_list`, `compozy__mcp_allow`, `compozy__mcp_import` return the same bodies.
entry_points: compozy mcp list|allow|import; compozy session mcp; compozy__mcp_list; compozy__session_mcp_list; compozy__mcp_allow; compozy__mcp_import
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-mcp-project-server-approval; ET-runtime-mcp-inventory-copy
---

Covers E2E-011 (US-011).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below. Reuse the fixtures of ET-runtime-mcp-inventory-copy and ET-mcp-project-server-approval.

Steps (record stdout, stderr and exit code of each):

1. `compozy mcp list --workspace <ws>`; `--runtime claude`; `--compozy-only`; `--runtime-only`;
   `-o json`; `--compozy-only --runtime-only` (error).
2. `compozy mcp allow sentry --workspace <ws>`; `--digest <stale>` (nonzero exit,
   `mcp_approval_definition_changed`); `--revoke`; a personal server (`mcp_approval_not_required`).
3. `compozy mcp import claude/advos`; `claude/plugin:figma:figma` (`mcp_import_unsupported_source`,
   exit 1); `codex/savee` twice (`mcp_import_name_taken`, then `--name savee-2`);
   a `cwd` server (`mcp_import_unsupported_definition`).
4. `compozy session mcp <id>` and `-o json` for a session in each state.
5. From a managed session call the four native tools with the matching inputs and diff their
   structured results against the CLI JSON.

Known shipped differences to accept rather than fail (see `_s9a-report.md`): the human `mcp list`
output has no column header, no CompozyOS STATUS/SOURCE columns and no group status phrase; runtime
IDs are `kimi-cli` and `qwen-code`; `session mcp` prints "Failed" for `failed`.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
