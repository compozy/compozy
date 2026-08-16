---
id: ET-agent-plugin-remote-header
area: ET
title: Authenticate a portable remote MCP server without exposing credentials
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A Vault-backed extension environment binding targets one declared streamable-HTTP server header, the daemon sends the resolved value only to that remote endpoint, and CLI, HTTP, UDS, native reads, logs, events, and diagnostics expose only key/server/header names and presence.
entry_points: compozy extension secrets bind <name> --env <key> --vault-ref <ref> --remote-header <server>:<header>; compozy extension secrets list; GET and PUT /api/extensions/:name/secrets over HTTP and UDS; https://compozy.com/docs/extensions/secrets; remote MCP invocation through a managed session
qa_status: pass
bug_ids: BUG-20260816-hosted-mcp-bootstrap-projection
fix_status: pending
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json; /Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/remote-mcp-requests.jsonl
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-ext-secrets-binding; ET-mcp-result-secret-redaction
---

QA impact 2026-08-16: extension secret bindings can now supply an operator-owned remote MCP header.
Task 08 must use a real authenticated endpoint, verify the header reaches only the named server, and
scan every public or retained surface for both the Vault reference and plaintext value.

QA 2026-08-16: unauthenticated initialize returned 401, the bound request returned 200, the remote MCP
invocation after unsetting the binding failed, and rebind recovered. Public reads exposed only the
environment key, MCP server, header name, and presence; Claude Code and Hermes both received the
authorized response without seeing the credential.
