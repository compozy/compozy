---
id: MS-web-mcp-editor-simple-advanced
area: MS
title: MCP server editor splits connection from process environment and locks route identity
persona: Dora
journey: J-mcp-authorize-repair
expected: Opening Add MCP server shows Simple only — a two-card choice between Local process and Streamable HTTP endpoint, the server name, and either the command or the endpoint URL. Advanced adds the process environment (args, env, typed secret inputs) for a local server, or the discovered OAuth registration block for a remote one — never both. Switching transport replaces the launch configuration: the abandoned branch's command, args, env, and secret_env are omitted from the request entirely. On edit, the server name and scope render as readable locked identity because they are the tool prefix agents already see. Untouched secret bindings emit their preservation flags and are never disclosed; a Vault-bound secret shows only its reference. The scope the definition writes to stays visible in both modes.
entry_points: web desktop shell → Marketplace → MCPs → Installed → Add server / Edit
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-07; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-08;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: MS-web-entity-modal-shell; ET-web-mcp-remote-editor; ET-web-marketplace-installed-management
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: no simple/advanced editor walk or independent saved-config read was retained.

story: As a person running agent work I add an MCP server by saying how it runs and where, and only open Advanced when the process needs arguments, environment, or OAuth.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.10–4.11), task_03, implemented 2026-07-25. Before this change every field was on one flat surface, the name was a disabled input on edit, and the transport was three equal cards even though `stdio` is the only local one.

The editor keeps its own dialog rather than routing through `SettingsEditorDialog`: the mode toolbar and the transport-dependent body are not part of that shell's contract, and nesting them would produce two chromes.

src: web/src/systems/settings/components/mcp-server-editor.tsx; web/src/systems/settings/components/mcp-editor-connection-section.tsx; web/src/systems/settings/components/mcp-editor-stdio-section.tsx; web/src/systems/settings/components/mcp-editor-oauth-section.tsx; web/src/systems/settings/lib/mcp-editor-model.ts

2026-08-20 qa-impact: Simple/Advanced sits on a recessed `--color-canvas-tint` chrome strip against the `--color-canvas-soft` shell. Status remains untested.
