---
title: Agents operate commands without the UI
type: feature
---

Everything the palette does is reachable from the CLI, HTTP, UDS, and native tools, with the same reasons and the same gates. An agent supervising CompozyOS discovers a command, checks its contract, targets a client, invokes it, and follows the approval — never depending on a browser. (#441)

- Native tools: `compozy__cmd_palette_list` reads the daemon-canonical catalog for the bound workspace, and `compozy__cmd_palette_invoke` runs one command with `id`, optional `args`, and optional `client`. Availability, targeting, single-flight, and approval rules all still apply.
- Every refusal is structured and carries the same text the UI row shows: `command_not_found`, `invalid_arguments` naming the fields, `no_attached_shell`, `multiple_clients` listing every attachment ID, and `already_running`.
- HTTP and UDS expose the catalog, clients, invocation, and stream under `/api/cmd-palette/*`, plus rank signals, usage, pins, and personalization. Approvals are read and canceled through `/api/tools/approvals/{id}`.
- Configuration parity is complete: bindings, aliases, pins, and personalization resets go through the same validated daemon paths Settings uses, and a change made by an agent reaches connected shells without a restart.
- `compozy approvals show|cancel` is a new top-level verb for the tool-approval lifecycle behind any invocation.

```bash
compozy cmd-palette invoke session.new --client <attachment-id> -o json
compozy cmd-palette invoke <destructive-id> -o json   # returns approval_pending + approval_id
compozy approvals show <approval-id> -o json          # pending → terminal
compozy approvals cancel <approval-id>
```
