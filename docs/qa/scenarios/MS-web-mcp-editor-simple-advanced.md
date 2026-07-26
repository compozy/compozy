---
id: MS-web-mcp-editor-simple-advanced
area: MS
title: MCP server editor splits connection from process environment and locks route identity
persona: Dora
journey:
expected: Opening Add MCP server shows Simple only — a two-card choice between Local process and Remote endpoint, the server name, and either the command or the endpoint URL. A remote endpoint exposes its wire transport (HTTP or SSE) inside the remote branch rather than as a third top-level card. Advanced adds the process environment (args, env, secret bindings) for a local server, or the OAuth block for a remote one — never both. Switching transport replaces the launch configuration: the abandoned branch's command, args, env, and secret_env are omitted from the request entirely. On edit, the server name and scope render as readable locked identity because they are the tool prefix agents already see. Untouched secret bindings emit their preservation flags and are never disclosed; a Vault-bound secret shows only its reference. The scope the definition writes to stays visible in both modes.
entry_points: web desktop shell → Marketplace → MCPs → Installed → Add server / Edit
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-07; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-08
last_report:
overlaps: MS-web-entity-modal-shell; ET-web-mcp-remote-editor; ET-web-marketplace-installed-management
---

story: As an operator I add an MCP server by saying how it runs and where, and only open Advanced when the process needs arguments, environment, or OAuth.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.10–4.11), task_03, implemented 2026-07-25. Before this change every field was on one flat surface, the name was a disabled input on edit, and the transport was three equal cards even though `stdio` is the only local one.

The editor keeps its own dialog rather than routing through `SettingsEditorDialog`: the mode toolbar and the transport-dependent body are not part of that shell's contract, and nesting them would produce two chromes.

src: web/src/systems/settings/components/mcp-server-editor.tsx; web/src/systems/settings/components/mcp-editor-connection-section.tsx; web/src/systems/settings/components/mcp-editor-stdio-section.tsx; web/src/systems/settings/components/mcp-editor-oauth-section.tsx; web/src/systems/settings/lib/mcp-editor-model.ts
