package mcp

import "errors"

// ErrAuthorizationRequired reports that a remote MCP server rejected the current bearer token.
var ErrAuthorizationRequired = errors.New("mcp: remote server requires authorization")

var errRemoteAuthenticationRejected = errors.New(
	"mcp: remote server rejected authentication without offering OAuth sign-in",
)
