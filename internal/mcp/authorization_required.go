package mcp

import "errors"

// ErrAuthorizationRequired reports that a remote MCP server rejected the current bearer token.
var ErrAuthorizationRequired = errors.New("mcp: remote server requires authorization")
