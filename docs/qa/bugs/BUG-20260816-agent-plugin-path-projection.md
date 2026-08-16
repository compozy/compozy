# BUG-20260816-agent-plugin-path-projection: Portable resources project invalid paths

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-extension-agent-authoring, validate and enable
- **Scenarios:** ET-agent-plugin-validation; ET-agent-plugin-conformance-walk
- **Found:** 2026-08-16 · **Report:** docs/qa/reports/2026-08-16-agent-plugins.md

## Summary

A conformant package could validate but fail during projection because the acquired package root used
the non-canonical macOS `/var` spelling and synthesized skill declarations stored absolute paths where
the extension manifest contract requires package-relative paths.

## Reproduction

- **Charter:** CH-agent-plugin-conformance · **Tour:** Garbage Tour
- **Environment:** macOS arm64, fresh isolated lab, local and pinned-git package sources

1. Validate, install, and enable the conformant `acme.tools` package.
2. Read its inventory and start a managed session.
3. Observe containment or manifest validation fail before the portable resources become available.

**Expected:** The package root is canonicalized once and synthesized skills remain relative to it.
**Actual:** Equivalent `/var` and `/private/var` roots failed containment, then absolute skill paths
failed the native manifest validator.

## Fix

- **Root cause:** Acquisition retained a lexical root while containment compared canonical paths; portable synthesis reused resolved filesystem paths instead of the validated relative declaration.
- **Fix commit:** pending the task 08 remediation checkpoint
- **Regression suites:** `internal/extension/manifest_test.go`; `internal/extension/tool_provider_test.go`

## Verification

- **Retested:** 2026-08-16 against the rebuilt isolated-lab binary.
- **Result:** Pass — status exposed two skills and two MCP servers, and both real provider walks loaded the same portable skill and stdio server.
- **Evidence:** `docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json`; Compozy sessions `sess-881a3c777fe36d33` and `sess-718859a9a9963f8a`.
