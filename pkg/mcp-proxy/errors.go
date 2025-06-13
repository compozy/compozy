package mcpproxy

import "errors"

// Sentinel errors for typed error handling
var (
	ErrNotFound           = errors.New("MCP not found")
	ErrAlreadyExists      = errors.New("MCP already exists")
	ErrHotReloadFailed    = errors.New("MCP definition updated but connection failed")
	ErrClientNotConnected = errors.New("MCP client not connected")
	ErrInvalidDefinition  = errors.New("invalid MCP definition")
	ErrStorageError       = errors.New("storage operation failed")
	ErrProxyRegFailed     = errors.New("proxy registration failed")
)
