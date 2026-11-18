---
id: MS-web-mcp-editor-simple-advanced
area: MS
title: MCP server editor leads with name and connection, folds the rest under More options, and locks route identity
persona: Dora
journey: J-mcp-authorize-repair
expected: Opening Add an MCP server shows "Name and where to use it" first (the name input with no separate label, and All projects / Only <project> title-only cards when a project is known), then "How it connects" — title-only Remote server / Local program cards and one source field. Until a card is picked the field reads "URL or command": an http(s) address becomes a remote server, anything else a command split into command and arguments, and the name follows the source until the person types one. "Paste JSON instead" accepts the mcpServers JSON from a server's docs: one server fills the form ("Found 1 server in the JSON and filled in the rest."), several let the person pick, and unreadable JSON leaves the draft untouched. A preview card shows what will be added. A local program's secret keys get their own "Key it needs" section (write-only SecretField, Vault binding, kept by presence on edit). Every other field — arguments, other settings, key names, OAuth, where it's saved, scope/source facts — lives in the framed More options fold. No field errors show until a field is blurred or edited; Add server stays disabled while invalid; a transport switch replaces the launch configuration and the abandoned branch's command, args, env and secret_env are omitted from the request. On edit, the name input and scope cards are locked because they are the tool prefix agents already see, and Remove server sits on the footer's leading edge.
entry_points: web desktop shell → Settings → MCP servers → Add MCP server / Edit; Marketplace → MCPs → Add MCP server
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-07; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-08;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-entity-modal-shell; ET-web-mcp-remote-editor; ET-web-marketplace-installed-management
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: no simple/advanced editor walk or independent saved-config read was retained.

story: As a person running agent work I add an MCP server by naming it, pasting its URL, command or docs JSON, and only open More options when the process needs arguments, settings, or OAuth.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.10–4.11), task_03, implemented 2026-07-25. Before this change every field was on one flat surface, the name was a disabled input on edit, and the transport was three equal cards even though `stdio` is the only local one.

The editor keeps its own dialog rather than routing through `SettingsEditorDialog`: the transport-dependent body is not part of that shell's contract.

src: web/src/systems/settings/components/mcp-server-editor.tsx; web/src/systems/settings/components/mcp-editor-connection-section.tsx; web/src/systems/settings/components/mcp-editor-json-paste.tsx; web/src/systems/settings/components/mcp-editor-more-options.tsx; web/src/systems/settings/components/mcp-secret-binding.tsx; web/src/systems/settings/lib/mcp-editor-paste.ts; web/src/systems/settings/components/mcp-editor-stdio-section.tsx; web/src/systems/settings/components/mcp-editor-oauth-section.tsx; web/src/systems/settings/lib/mcp-editor-model.ts

2026-08-20 qa-impact: Simple/Advanced sits on a recessed `--color-canvas-tint` chrome strip against the `--color-canvas-soft` shell. Status remains untested.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

2026-11-07 (#1337): required-field errors (Name, Command, URL) now wait for blur or edit; a transport switch resets the cleared launch fields to pristine. Verified in the real web app against the live daemon: pristine Local program and Remote URL show no errors; blurring empty Name and URL shows both errors; switching back to Local program keeps Command pristine; a filled form enables Add server; Edit opens without errors. Only this slice was walked; the scenario status stays as recorded above.

2026-11-16 qa-impact (MCP UX redesign S3): the Simple/Advanced toolbar is replaced by the prototype layout (`web/src/drafts/mcp-settings-redesign/server-dialogs.tsx`): name and scope first, one URL-or-command field with JSON paste and a preview, "Key it needs" with SecretField, and a framed More options fold whose trigger keeps the `settings-mcp-servers-editor-mode-advanced` test id. A new server can now pick All projects or Only <project>; the name check reads the picked scope's own list. Covered by focused component, model and Settings page tests; not walked in the real app, so the status stays as recorded above.
