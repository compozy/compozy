package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	protocolVersionModern = "2026-07-28"
	protocolVersionLegacy = "2025-11-25"
)

// UnsupportedProtocolVersionError reports a peer that negotiated outside Compozy's MCP contract.
type UnsupportedProtocolVersionError struct{ Version string }

func (e *UnsupportedProtocolVersionError) Error() string {
	return fmt.Sprintf("mcp: negotiated unsupported protocol version %q", e.Version)
}

func supportedProtocolVersion(version string) bool {
	return version == protocolVersionModern || version == protocolVersionLegacy
}

type protocolRestrictedTransport struct{ mcpgo.Transport }

func (protocolRestrictedTransport) SupportsProtocolVersion(version string) bool {
	return supportedProtocolVersion(version)
}

func protocolVersionMiddleware() mcpgo.Middleware {
	return func(next mcpgo.MethodHandler) mcpgo.MethodHandler {
		return func(ctx context.Context, method string, request mcpgo.Request) (mcpgo.Result, error) {
			if method == "initialize" {
				if initialize, ok := request.(*mcpgo.ServerRequest[*mcpgo.InitializeParams]); ok && initialize.Params != nil && !supportedProtocolVersion(initialize.Params.ProtocolVersion) {
					return nil, &UnsupportedProtocolVersionError{Version: initialize.Params.ProtocolVersion}
				}
			}
			result, err := next(ctx, method, request)
			if err != nil || method != "server/discover" {
				return result, err
			}
			if discover, ok := result.(*mcpgo.DiscoverResult); ok {
				versions := discover.SupportedVersions[:0]
				for _, version := range discover.SupportedVersions {
					if supportedProtocolVersion(version) {
						versions = append(versions, version)
					}
				}
				discover.SupportedVersions = versions
			}
			return result, nil
		}
	}
}
