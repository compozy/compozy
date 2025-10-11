---
title: Compozy Native Tools (cp__*)
description: Built-in, sandboxed tools for reliable local execution
---

Overview

- Compozy ships built-in, sandboxed tools exposed with the cp\_\_ prefix.
- These tools replace legacy @compozy/tool-\* npm packages.
- Benefits: zero external runtime, uniform error catalog, consistent telemetry.

Core Tools

- cp**read_file, cp**write_file, cp**delete_file, cp**list_dir, cp\_\_grep
- cp\_\_exec (allowlisted absolute paths, capped stdout/stderr, timeouts)
- cp\_\_fetch (HTTP(S) only, size and redirect caps)
- cp\_\_agent_orchestrate (inline multi-agent planning and execution)
- cp\_\_list_agents, cp\_\_describe_agent (agent catalog discovery helpers)

Configuration

- Configure via application config: NativeTools.RootDir, NativeTools.Exec.Allowlist
- Use config.FromContext(ctx) in code paths; never global singletons.

Observability

- Each invocation emits structured logs with tool_id and request_id.
- Prometheus metrics: compozy_tool_invocations_total, compozy_tool_latency_seconds, compozy_tool_response_bytes.
- Errors map to canonical codes: InvalidArgument, PermissionDenied, FileNotFound, CommandNotAllowed, Internal.

Migration Guide

- Replace imports of @compozy/tool-\* with cp\_\_ tool IDs in agent/tool configs.
- Remove Bun workspace dependencies for tools in package.json.
- Update docs/examples to reference cp\_\_ identifiers exclusively.

Troubleshooting

- PermissionDenied: check sandbox root and allowlist settings.
- InvalidArgument: verify schema fields (path/url/method, etc.).
- Timeouts: adjust per-tool timeout_ms within configured caps.
