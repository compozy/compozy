# BUG-20260910-hosted-mcp-text-result: Managed Cursor cannot read complete native tool results

- **Status:** fixed
- **Impact (user-side):** Task-Block
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, inspect target and coordination state
- **Scenarios:** ET-workspace-access-mode-matrix (partial walk)
- **Found:** 2026-09-10 · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and evidence

In the isolated lab, managed Cursor Grok 4.6 High Fast session `sess-35af9e34a61e15e4` inspected the foreign workspace and ran a terminal coordination read. Cursor received summaries without the complete JSON/output. The earlier Cursor canary also needed a provider file-read fallback after the Compozy terminal read. Evidence under `docs/qa/evidence/2026-09-10-qa-execution-unblock/`: `cross-all-prompt.json`, `cross-all-message.json`, `cursor-canary-prompt.json`, and tool event records.

Expected: the authorized structured result remains available to clients consuming MCP text content. Actual: `hostedToolResult` chooses Preview for text whenever Structured is present; only `structuredContent` contains the full result. The persona walk ended before source diagnosis. The separate spawn schema failure remains under investigation and is not attributed to this text omission.

## Correction and validation

The MCP adapter preserves the existing preview and additionally emits the same bounded structured JSON as a text block. Without a preview the existing JSON fallback remains sufficient. The canonical `TestHostedProxyHelpers` regression failed before the change (`mcp-text-red.txt`); the full MCP race suite passed afterward (`mcp-text-green.txt`). `make gate` passed all affected lanes (`mcp-text-gate.txt`). Fresh managed Cursor session `sess-ab20fe5bde29aefc` returned the exact independently checked file content and the foreign workspace name/path (`mcp-text-retest-proof.json`, `mcp-text-retest-runtime.json`). The target file was read through native `terminal_exec`; Cursor separately read two of its own skill-result artifact files. Session stop was verified (`mcp-text-retest-stop.json`). The complete cross-workspace mode matrix remains pending.

The [MCP tools specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#structured-content) recommends serialized JSON text alongside structured content for compatibility.

## Cross-surface impact

- Native tools/MCP: additive text projection of the already authorized result; IDs, descriptors, schemas, policy, and structured content remain unchanged. No public replacement or compatibility shim.
- Extensibility/hooks/config: MCP consumers gain readable JSON; no hook, config, extension SDK or registration changes.
- Workspace isolation: scope authorization and bounded result/artifact handling occur upstream and remain unchanged. No new reads or persisted shape changes.
- Official skill: tools-and-skills reference explains the text projection. Existing native tool invocation instructions remain valid.
- Web/Docs: Web consumes the existing canonical tool result, not this MCP text adapter. No Web DTO or route change. This bug and the affected QA scenario record the diagnostic and retest limits.

## PR integration correction

PR #624 portable-extension E2E exposed duplicate text blocks when the preview already contains the complete structured JSON. The hosted adapter now avoids appending that identical payload twice (comparing decoded JSON values, including reordered object keys), while retaining a distinct preview plus complete JSON when the preview is a summary. The existing hosted-proxy helper suite covers both branches; the portable-extension daemon E2E retains its one-result contract.
