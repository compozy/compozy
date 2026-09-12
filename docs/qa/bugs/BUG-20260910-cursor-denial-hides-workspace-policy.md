# BUG-20260910-cursor-denial-hides-workspace-policy: Restricted Cursor sessions lose the workspace denial diagnostic

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, understand why archive operations are refused
- **Scenarios:** ET-workspace-access-mode-matrix
- **Found:** 2026-09-10 (local time) · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Observed reproduction

Managed session `sess-bc315ce9f46d87a7` uses field-deny in field-notes, permissions.mode deny-all, Cursor Grok4.6 High Fast. Ada requests native workspace_info/memory_list against field-archive, then native terminal_exec for the documented agent CLI task next, spawn, coordination status, and peers commands. The agent attempts every requested operation and settles. No operator approval was offered or answered; the public pending-interactions read is empty.

All operations are refused before the target handler: recorded permission events have `decision: reject-once`, `resolved_by: provider`; the provider reports `User rejected MCP: ... - User rejected`. Even same-workspace skill_view and tool_info are refused. The six requested operations return no daemon workspace_access_denied reason, prescribed permission-mode hint, or CLI exit77. Operator audit query for workspace.access_denied in this session returns an empty list.

Evidence under `docs/qa/evidence/2026-09-10-qa-execution-unblock/`: `cross-deny-new.json`, `cross-deny-prompt.json`, `cross-deny-events-final.json`, `cross-deny-interactions.json`, `cross-deny-audit.json`, and `cross-deny-stop.json`.

## Boundary and disposition

The permission boundary prevented execution; this is not evidence of an access bypass or unauthorized mutation. It does contradict the scenario's error-observability expectation for this real provider: the workspace policy never evaluates the rejected requests. Agent CLI commands were not launched, so their absent exit codes cannot be counted as exit77 passes. HTTP/UDS agent-driven branches cannot be reached via the same rejected terminal without changing the setup.

No product source diagnosis or alternate provider execution is claimed. This differs from BUG-20260910-cursor-mcp-error-schema, which masks an error after the daemon tool has executed. Here rejection precedes the handler. A resolution must retain the deny-all execution boundary while making the actual denial source and actionable guidance available, or explicitly define the provider-level preemption in the public contract. Do not auto-allow tool execution, impersonate an agent from the operator, or change the selected provider to manufacture the expected workspace audit.

The known approve-all repaired paths remain independently valid. Approve-reads and additional supported seams continue separately; this observed failure is not a whole-matrix pass.
