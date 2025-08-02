# Compozy MCP Proxy

This is a separate Go module for the MCP (Model Context Protocol) proxy component of Compozy.

## Module Structure

This directory contains its own `go.mod` file to enable independent versioning and releases through Release Please. It imports shared packages from the main Compozy module using Go's `replace` directive.

## Building

The MCP proxy is built using the dedicated `.goreleaser.mcp-proxy.yml` configuration file in the repository root.

## Dependencies

All internal dependencies are resolved through the replace directive in `go.mod`:
```
replace github.com/compozy/compozy => ../..
```

This ensures that local development uses the current code while maintaining proper module boundaries for versioning.