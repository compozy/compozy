# Config Path Error Message Design

## Problem

`compozy__config_get` correctly classifies an absent configuration entry with the public
`config_path_not_found` reason, but the HTTP/UDS tool-error serializer chooses its safe message only
from the broader `tool_not_found` code. The hosted MCP proxy then exposes the contradictory text
`config_path_not_found: tool not found` to managed agents. Any authored, packaged, or
extension-published agent can therefore mistake an absent value for a missing tool.

Issue: [#394](https://github.com/compozy/compozy/issues/394).

## Contract

An absent path keeps the existing envelope:

- error code: `tool_not_found`;
- reason code: `config_path_not_found`;
- HTTP status: `404`;
- ToolID and HTTP/UDS/MCP schemas: unchanged.

Only the safe public message becomes reason-aware. For `config_path_not_found`, transports expose
`config path not found`; the hosted MCP result therefore becomes
`config_path_not_found: config path not found`. It must not contain `tool not found`.

Every other reason continues to use the existing code-derived safe message unless it receives its
own explicitly reviewed mapping in a future change. Internal error strings remain masked.

## Implementation

Extend the existing safe-message selector in `internal/api/core/tool_errors.go` to receive reason
codes. It checks for the one public reason-specific mapping before falling back to the existing
error-code switch. This fixes the source transport envelope; the hosted MCP proxy already combines
the primary reason with that public message and needs no production special case.

## Verification

- A core API test proves the stable code, reason, status, and reason-aware message.
- A hosted MCP proxy test passes the real serialized response through the existing projection and
  asserts the exact provider-visible text.
- The existing extension-agent daemon E2E uses a neutral absent Loop-input path, asserts the exact
  hosted MCP result, and forbids `tool not found`.
- Existing tool-error tests prove unrelated codes retain their current messages.

## Impact

- Native tools: no ToolID, descriptor, schema, risk, capability, or reason-code change.
- Extensibility: every installed extension receives the same corrected hosted error; no
  extension-specific behavior is introduced.
- Workspace isolation: unchanged; the existing session workspace binding remains authoritative.
- Config lifecycle: no configuration value is written or renamed.
- Web and generated contracts: unchanged.

## Out of Scope

No new public code or reason, no prompt workaround, no Batuta-specific fallback, and no general
rewrite of tool-error messages.
