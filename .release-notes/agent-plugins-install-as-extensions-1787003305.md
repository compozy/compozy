---
title: Agent Plugins install as extensions
type: feature
---

CompozyOS ingests [Agent Plugins 1.0.0](https://agent-plugins.org/) packages as extensions with no Compozy-specific manifest. A portable plugin contributes skills plus local or remote MCP servers while keeping the existing extension lifecycle, trust, isolation, diagnostics, Marketplace, CLI, HTTP, UDS, native-tool, and Web management surfaces. (#419)

- Manifest discovery and validation are strict: fixed-location skills and MCP configuration, closed schemas, deterministic diagnostics, native-manifest precedence, and safe rejection of unsupported components.
- Portable skills and stdio or streamable-HTTP MCP servers are synthesized into the canonical extension model, with absolute `PLUGIN_ROOT` and `PLUGIN_DATA` expansion, single-token stdio commands, package-root working directories, remote-header bindings, URL policy, and secret redaction.

Migration notes: end-to-end delivery is claimed only for the provider paths proven end to end, Claude Code and Hermes. OpenClaw's current ACP bridge advertises `session_mcp=false`, so CompozyOS fails closed instead of pretending to deliver session MCP servers.
