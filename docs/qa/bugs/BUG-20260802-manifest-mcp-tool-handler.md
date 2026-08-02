# BUG-20260802-manifest-mcp-tool-handler: MCP-backed manifest tools required an extension-host handler

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, validate a complete code-first kit
- **Scenarios:** ET-extension-code-first-authoring; ET-ext-kit-enable; ET-ext-inventory
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 real-user QA

## Summary

A manifest tool with the supported `mcp` backend and complete `server` plus `tool` binding failed
validation with `handler_missing`. The validator incorrectly applied the extension-host runtime
descriptor contract to an MCP-backed tool, so a complete static kit could not declare that tool.

## Reproduction

1. Declare `resources.tools.catalog_probe.backend.kind` as `mcp`.
2. Supply both `server: bundles-removal-mcp` and `tool: catalog_probe`.
3. Run `compozy extension validate <fixture> -o json`.

**Expected:** Validation uses the MCP `server + tool` binding and succeeds without a subprocess
handler.
**Actual:** Validation fails with `handler_missing: handler is required`.

## Evidence

- Red canonical test:
  `CGO_ENABLED=1 go test -race ./internal/extension -run '^TestResolveManifestToolDescriptorsIncludesDigestAndMetadata$' -count=1`.
- The pre-fix test failed only in `Should_Resolve_MCP_Backend_Without_Extension_Host_Handler`.
- The rebuilt CLI validated the 1.1.0 fixture with zero issues, then inventory projected its tool,
  Loop, layout, agent sidecars, automation, hook, MCP server, and skill as one 12-item kit.

## Fix

- **Root cause:** `manifestRuntimeDescriptor` always validated an
  `ExtensionToolRuntimeDescriptor`, whose handler is mandatory only for `extension_host` tools.
- **Correction:** Runtime-descriptor validation now runs only for the extension-host backend. The
  canonical `Tool` descriptor continues to validate MCP backend, schema, risk, source, and identity.
- **Fix commit:** `881a254`
- **Regression test:** `TestResolveManifestToolDescriptorsIncludesDigestAndMetadata/Should Resolve MCP Backend Without Extension Host Handler`.

## Verification

- The focused regression and the complete `internal/extension` package passed under `-race`.
- The rebuilt CLI validated and enabled the complete fixture; inventory reported all 12 items live.

