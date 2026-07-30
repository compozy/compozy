package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	protocolInitializeMethod      = "initialize"
	protocolVersionModern         = "2026-07-28"
	protocolVersionLegacy         = "2025-11-25"
	protocolVersionProviderLegacy = "2025-06-18"
)

// UnsupportedProtocolVersionError reports a peer that negotiated outside Compozy's MCP contract.
type UnsupportedProtocolVersionError struct{ Version string }

func (e *UnsupportedProtocolVersionError) Error() string {
	return fmt.Sprintf("mcp: negotiated unsupported protocol version %q", e.Version)
}

func supportedProtocolVersion(version string) bool {
	return version == protocolVersionModern || version == protocolVersionLegacy
}

// supportedHostedProviderProtocolVersion keeps the daemon's private provider bridge compatible
// with managed agent runtimes. It does not widen the public MCP client or server contract.
func supportedHostedProviderProtocolVersion(version string) bool {
	return supportedProtocolVersion(version) || version == protocolVersionProviderLegacy
}

type protocolRestrictedTransport struct{ mcpgo.Transport }

func (protocolRestrictedTransport) SupportsProtocolVersion(version string) bool {
	return supportedProtocolVersion(version)
}

type hostedProviderProtocolTransport struct{ mcpgo.Transport }

func (hostedProviderProtocolTransport) SupportsProtocolVersion(version string) bool {
	return supportedHostedProviderProtocolVersion(version)
}

func protocolVersionMiddleware() mcpgo.Middleware {
	return protocolVersionMiddlewareFor(supportedProtocolVersion)
}

func hostedProviderProtocolVersionMiddleware() mcpgo.Middleware {
	return protocolVersionMiddlewareFor(supportedHostedProviderProtocolVersion)
}

func protocolVersionMiddlewareFor(supports func(string) bool) mcpgo.Middleware {
	return func(next mcpgo.MethodHandler) mcpgo.MethodHandler {
		return func(ctx context.Context, method string, request mcpgo.Request) (mcpgo.Result, error) {
			if method == protocolInitializeMethod {
				initialize, ok := request.(*mcpgo.ServerRequest[*mcpgo.InitializeParams])
				if ok && initialize.Params != nil &&
					!supports(initialize.Params.ProtocolVersion) {
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
					if supports(version) {
						versions = append(versions, version)
					}
				}
				discover.SupportedVersions = versions
			}
			return result, nil
		}
	}
}
