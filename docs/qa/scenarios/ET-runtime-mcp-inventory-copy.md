---
id: ET-runtime-mcp-inventory-copy
area: ET
title: Settings lists every runtime's own MCP servers read-only and copies one into CompozyOS
persona: Dora
journey:
expected: With Claude/Codex/Pi fixture configs (and fixtures for all 10 runtimes) in the lab home, Settings → MCP servers shows read-only groups per runtime with the runtime filter; Copy to CompozyOS on Claude `advos` adds a CompozyOS row, stores values in Vault, leaves the runtime file byte-identical, and refuses plugin/SSE/cwd rows with their reasons. The inventory never runs a runtime, connects to a server or reads a keychain.
entry_points: web Settings → MCP servers and Marketplace → MCPs (runtime filter, Add to CompozyOS panel beside Copy to CompozyOS, Show file); compozy mcp list; compozy mcp import <runtime>/<server>; GET /api/settings/runtime-mcp-servers; POST /api/settings/runtime-mcp-servers/{runtime}/{name}/import
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-cli-mcp-runtime-verbs; RT-runtime-mcp-switch
---

Covers E2E-007 and the inventory half of E2E-008 (US-007, US-008).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.
Fixtures: lab home `~/.claude.json` with `advos` (HTTP, `Authorization` header), `cua-driver`
(stdio), one plugin server and one server with `cwd`; `~/.codex/config.toml` with `savee` and a
disabled `computer-use`; `~/.pi/agent/mcp.json` with `rea`; plus one fixture file each for Gemini CLI,
OpenCode, Kimi CLI, Qwen Code, Copilot CLI, Cursor Agent and Antigravity (installed inside the lab
only, decision Q7 = a). Record `sha256sum` of every fixture file before the walk.

Steps:

1. Settings → MCP servers: CompozyOS group first, then groups "Claude Code", "Codex", "Pi" and the
   other runtimes; each header names the runtime, count and "Loaded in CompozyOS sessions" or
   "<Runtime> always loads its own servers". Env/header key names show, values never. Screenshot.
2. Filter "Codex": only the Codex group remains. Screenshot. Repeat in Marketplace → MCPs.
3. Corrupt `~/.claude.json` temporarily: only the Claude group shows "Couldn't read ~/.claude.json: …";
   others unaffected. Restore.
4. Copy `advos` to CompozyOS (Personal). Toast "advos copied. Sessions can use it from their next
   message."; the CompozyOS group lists `advos` (highlighted), needs sign-in. Vault holds the header
   value (`compozy vault list` shows the ref, never the value).
5. The plugin row and the `cwd` row show no Copy button and their reason.
6. Copy `savee` twice: the second shows the inline name-taken error; copying with a new name succeeds.
7. Re-hash every fixture file: unchanged. Process list during the walk shows no runtime CLI or MCP
   server started by the inventory.

Pass: steps 1–7; no secret value in any DTO, log line or screenshot.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md

QA impact 2026-11-14 review fixes: follow a symlinked runtime config safely without executing it. List with `compozy mcp list --workspace <project>` and confirm the workspace lens. Edit an observed runtime definition before Copy: expect “changed in <source>. List it again before copying”; removal still says “no longer in <source>”. Exercise import output with `-o json`, `-o jsonl`, and `-o toon`. Public definition digests remain stable after daemon restart and differ across installations. Focused backend regressions cover these boundaries; the full live scenario re-walk remains an integration responsibility.

QA impact 2026-11-14 keyed digest consistency: list a runtime server, retain its digest,
restart the isolated daemon with the same home and copy the unchanged definition. The
new inventory and import source must retain the listed digest. Confirm an edited source
still requires a fresh list. The daemon must fail initialization if its installation digest
key is unreadable, rather than substituting a process key. This repair's live re-walk remains
unverified under the operator's CI-only directive.

QA impact 2026-11-16 (MCP redesign S2): Copy to CompozyOS opens the "Add <server> to CompozyOS"
panel beside the button instead of a dialog: Use it in All projects (Personal) · This profile (named
profile lens) · Only this project (needs a project in view), the vault note for its keys, and Add to
CompozyOS. A taken name (409) keeps the panel open and shows "Name in CompozyOS" with the error; a
source gone from its file warns inside the panel and blocks another try.
