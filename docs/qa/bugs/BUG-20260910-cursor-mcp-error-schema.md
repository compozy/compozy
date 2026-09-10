# BUG-20260910-cursor-mcp-error-schema: Cursor masks native tool failures with output-schema validation

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, understand a refused spawn
- **Scenarios:** ET-workspace-access-mode-matrix (adjacent diagnostic)
- **Found:** 2026-09-10 · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

Fresh managed Cursor Grok 4.6 High Fast session `sess-4cbe38a56cc29a66` inspected `compozy__session_spawn`, then called it once for nonexistent agent `qa-missing-agent-20260910`, TTL 60 seconds, auto-stop enabled, creator notification disabled. No foreign workspace was requested. The turn completed without retry and the session was verified stopped.

Expected: a readable native error identifying why the spawn was refused. Actual: MCP error -32602 saying structured content lacks the four required spawn success fields and has additional properties. This independently reproduces the earlier masked cross-workspace attempt. Evidence: `mcp-error-retest-prompt.json`, `mcp-error-retest-events-final.json`, `mcp-error-retest-stop.json` in `docs/qa/evidence/2026-09-10-qa-execution-unblock/`.

## Diagnosis and boundary

Compozy `callHostedTool` sets `IsError: true` and preserves its typed public error envelope in `StructuredContent`; `hostedMCPTool` preserves the canonical descriptor's success output schema. The Cursor diagnostic matches the upstream TypeScript MCP client validation issue [#1943](https://github.com/modelcontextprotocol/typescript-sdk/issues/1943), where structured error content is validated against the success schema despite `isError`. The upstream [proposed client correction #1945](https://github.com/modelcontextprotocol/typescript-sdk/pull/1945) skips success validation for error results. These sources establish the known interop pattern, not the exact embedded SDK version in Cursor.

No Compozy schema was relaxed and no structured error field was removed. Those changes would alter the existing public contract and require a broader compatibility design, including schema reference preservation and strict success validation. No Cursor installation/dependency was patched. The current Cursor build remains affected in the real run; its embedded SDK version has not been independently established.

Recommended resolution: a provider client carrying the upstream error-validation correction, or an explicitly designed lossless MCP error-schema contract. Continue supported CLI/HTTP/UDS branches and report this exact native diagnostic limitation; do not award the affected error-observability branch a pass.
