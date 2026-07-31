# QA Plan — 2026-07-30 — MCP 2026/catalog v2

- **Scope:** MCP 2026-07-28 runtime hard cut, Streamable HTTP and stdio interoperability, discovered OAuth/DCR/CIMD, marketplace manifest v2 and typed inputs, and their public CLI, API, Web, site, native-tool, official-skill, and workspace-host projections.
- **Cadence tier:** full — protocol, credential, catalog, and public-contract migration.
- **Status:** planning only. No runtime was launched, no persona walk occurred, and this file records no verdict or evidence.
- **Execution rule:** create the execution report from the local template before the first session; update this plan only if scope changes before execution.

## Execution matrix (ordered by risk)

| Order | Charter | Persona | Journey | Scenarios to settle | Isolated-walk prerequisite | Planned outcome |
|---:|---|---|---|---|---|---|
| 1 | CH-mcp-protocol-interop | Ada | J-mcp-protocol-interop | ET-mcp-protocol-interop; ET-workspace-host-api-mcp; ET-compozy-native-tool-invocation | Fresh daemon; shared official-SDK fixture profiles for 2026-07-28, 2025-11-25, stateless HTTP, and unsupported capability; two targets and two workspaces | Public stdio/Streamable HTTP transcript, negotiated-version status, cache-boundary observations, independent read path |
| 2 | CH-mcp-authorize-repair-truth | Bruno | J-mcp-authorize-repair | ET-047; ET-api-mcp-oauth-endpoints; ET-web-mcp-authorize; ET-web-mcp-authorize-manual; ET-web-mcp-status-matrix; MS-029 | Fresh OAuth issuer supporting discovery, CIMD, DCR, step-up, mix-up, refresh and redirect validation; loopback callback; browser; redaction log capture | Auth lifecycle evidence showing pre-registration → CIMD → one DCR fallback, full redirect-only manual exchange, and scope/target binding |
| 3 | CH-agent-marketplace-parity | Ada | J-agent-marketplace-parity | ET-046; ET-api-marketplace-namespace; ET-api-mcp-catalog-install; ET-cli-marketplace-info; ET-cli-marketplace-refresh; ET-cli-marketplace-search; ET-cli-mcp-install; ET-049 | Real v2 MCP feed with Context7 plus representative stdio npm/uvx/docker and hosted entries; declared inputs; Vault; HTTP, UDS and CLI access | Cross-plane install/search/detail parity, typed input redaction, manifest-v1 rejection, scope-default and launch-isolation proof |
| 4 | CH-marketplace-under-a-minute | Bruno | J-marketplace-acquisition | ET-web-mcp-guided-install; ET-web-marketplace-installed-management; ET-web-marketplace-mcp-authorize-installed; ET-web-mcp-remote-editor; MS-web-mcp-editor-simple-advanced | Isolated Web QA with COMPOZY_WEB_API_PROXY_TARGET from bootstrap manifest; real v2 feed; browser; Vault; OAuth target | Screen and fresh-read confirmation of v2 inputs, no SSE UI, truthful authorization handoff, and no secret disclosure |
| 5 | CH-site-docs-marketplace-truth + CH-compozy-platform-hard-cut | Dora / Ada | J-evaluate-compozy-beta / J-validate-compozy-hard-cut | ET-site-marketplace-catalog; ET-compozy-official-skill-discovery | Built site from the final feeds and docs; fresh runtime with bundled official skill; public CLI/API read paths | Site v2 schema and omission claims agree with feeds; official skill gives only current MCP v2 paths; legacy config/flags/SSE are absent |

## Required environment before execution

1. Run `eng-qa-bootstrap` to mint a fresh isolated lab and export its `COMPOZY_HOME`, daemon URLs, provider-home values, `COMPOZY_WEB_API_PROXY_TARGET`, and teardown command. Do not use default home, port, or provider state.
2. Build the final implementation and pass the automated gate required by the workstream before any persona session. The future report records the exact gate evidence.
3. Supply a live local test issuer and catalog fixtures that exercise discovery, CIMD, DCR, redirect validation, step-up, refresh, v2 manifest rejection, typed inputs, and each launch distribution. Do not replace these with internal calls or static mocks.
4. Start any registered lab process through the bootstrap envelope, collect checkpoint/failure evidence under its QA output path, and always run `eval "$TEARDOWN_COMMAND"`; execution may close only with `teardown.json` reporting `clean: true`.

## Coverage decisions

- The matrix deliberately separates standard-client protocol interoperability, OAuth security, structured catalog operations, human Web acquisition, and published guidance. This avoids accepting a green UI or unit suite as evidence that an ordinary MCP client can interoperate.
- Catalog vendor breadth is not a per-vendor test suite. The v2 feed uses representative distribution/input/auth shapes; the matrix checks the generic contract and the publication list through the real final catalog.
- No verdict is assigned here. `qa_status: untested` marks changed scenarios until the execution contract records a real walk, evidence, and report.
