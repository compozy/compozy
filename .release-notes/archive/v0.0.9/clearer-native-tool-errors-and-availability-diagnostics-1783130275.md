---
title: Clearer native tool errors and availability diagnostics
type: fix
---

Native and hosted tool calls now fail legibly. Hosted MCP tool calls return richer, structured JSON error details — the tool ID, an error code, and any denial reasons — instead of opaque failures, and advertised tools carry improved metadata and descriptions. When runtime diagnostic data cannot be retrieved, native tool lookups report a clear "unavailable" diagnostic rather than a silent miss. Hosted and session MCP availability handling is safer overall, with clearer informational and warning logs and graceful fallbacks when a feature is disabled. Documentation and the runtime envelope now consistently direct agents to the canonical `agh__*` tool IDs and the harness-returned tool references.
