---
id: ET-agent-plugin-remote-header
area: ET
title: Authenticate a portable remote MCP server without exposing credentials
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A Vault-backed extension environment binding targets one declared streamable-HTTP server header, the daemon sends the resolved value only to that remote endpoint, and CLI, HTTP, UDS, native reads, logs, events, and diagnostics expose only key/server/header names and presence.
entry_points: compozy extension secrets bind <name> --env <key> --vault-ref <ref> --remote-header <server>:<header>; compozy extension secrets list; GET and PUT /api/extensions/:name/secrets over HTTP and UDS; https://compozy.com/docs/extensions/secrets; remote MCP invocation through a managed session
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-secrets-binding; ET-mcp-result-secret-redaction
---

QA impact 2026-08-16: extension secret bindings can now supply an operator-owned remote MCP header.
Task 08 must use a real authenticated endpoint, verify the header reaches only the named server, and
scan every public or retained surface for both the Vault reference and plaintext value.
