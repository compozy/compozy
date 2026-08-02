---
id: ET-mcp-result-secret-redaction
area: ET
title: Remove MCP credentials from descriptors and tool results
persona: Ada
journey: J-mcp-authorize-repair
expected: A remote or stdio MCP server cannot reflect a request-scoped OAuth bearer or SecretEnv credential through tool descriptors, cached/provider projections, errors, logs, result text, metadata, JSON keys, structured content, image/audio/blob bytes, base64 or URL-safe base64, hexadecimal text, or exact/embedded percent-encoded data URIs. Valid credential escapes remain protected even when unrelated malformed percent text surrounds them. Credentials of every nonblank length are removed before the request-scoped registrar is released; post-redaction JSON-key collisions fail closed; unrelated application bytes remain unchanged; and cleanup removes the credential from the redactor exactly once.
entry_points: MCP tools/list discovery and cache; compozy__mcp__* tool invocation; HTTP OAuth bearer; stdio secret_env; CLI/HTTP/UDS/native tool result projections
qa_status: untested
bug_ids:
fix_status: fixed
retest_status: pending
fix_commits: pending final whole-diff commit
evidence: internal/redact/redact_test.go; internal/mcp/executor_test.go
last_report:
overlaps: ET-047; ET-api-mcp-oauth-endpoints; ET-compozy-native-tool-invocation
---

Added by the restarted Go audit after adversarial review found that a short MCP credential could be
reflected through reversible binary representations even when literal-string scans passed. The
current implementation applies exact request-scoped redaction before descriptors enter cache/provider
projection and before tool results leave the executor.

Run one authenticated HTTP MCP fixture with a short bearer and one stdio fixture with a short
`secret_env` value. Make each server reflect the credential across presentation metadata, nested
JSON keys and values, structured output, image/audio/blob content, and reversible encodings, including
lower/uppercase `%HH` escapes with an unrelated malformed `%` suffix. Invoke
discovery twice to exercise the cache, invoke the tool through the public agent/native-tool path, and
scan every returned payload, runtime log, event, and diagnostic. Prove fail-closed JSON-key collision
handling and verify that unrelated binary content remains byte-identical.

QA impact 2026-08-03: new security-regression owner. Fresh package-level race, stress, vet, lint,
Windows cross-build, convention, formatting, and diff evidence is green; a fresh isolated public
scenario remains required before the modernization workstream closes.
