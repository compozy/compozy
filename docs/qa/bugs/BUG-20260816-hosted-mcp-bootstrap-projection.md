# BUG-20260816-hosted-mcp-bootstrap-projection: Managed sessions lose portable MCP tools

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-extension-distribution, consume portable tools in a managed session
- **Scenarios:** ET-agent-plugin-provider-delivery; ET-agent-plugin-remote-header; ET-agent-plugin-conformance-walk
- **Found:** 2026-08-16 · **Report:** docs/qa/reports/2026-08-16-agent-plugins.md

## Summary

Fresh managed sessions intermittently started without explicitly requested portable MCP tools. A
second failure mode projected absent output schemas as JSON `null`, which Claude Code rejected during
its MCP `tools/list` validation.

## Reproduction

- **Charter:** CH-agent-plugin-provider-delivery · **Tour:** Feature Tour
- **Environment:** macOS arm64, one enabled package, real Claude Code and Hermes sessions

1. Enable a portable package with one stdio and one streamable-http MCP server.
2. Immediately create a managed session whose agent requests both tools.
3. Prompt the provider to invoke both tools.

**Expected:** Bootstrap waits for required discovery, publishes one stable projection generation, and omits an absent optional output schema.
**Actual:** Reconciliation could still be in flight, stable bootstrap state could be overwritten by an empty live projection, and Claude Code rejected `outputSchema: null`.

## Fix

- **Root cause:** Reconciliation had no idle fence for session bootstrap; required deferred tools did not trigger full discovery; bootstrap did not seed the stable generation cache; the wire mapper used a typed nil that JSON encoded as `null`.
- **Fix commit:** pending the task 08 remediation checkpoint
- **Regression suites:** `internal/resources/reconcile_test.go`; `internal/tools/registry_test.go`; `internal/mcp/hosted_test.go`; `internal/mcp/hosted_proxy_test.go`

## Verification

- **Retested:** 2026-08-16 with fresh Claude Code and Hermes sessions after rebuilding the QA binary.
- **Result:** Pass — both providers loaded the portable skill and invoked the canonical local and remote tools on their first final-code attempt.
- **Evidence:** `docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json`; Claude validation trace `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/claude-mcp-validation-0605.log`.
