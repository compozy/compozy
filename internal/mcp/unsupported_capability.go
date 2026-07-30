package mcp

import "fmt"

// UnsupportedCapabilityError reports an MCP capability Compozy intentionally does not implement.
type UnsupportedCapabilityError struct {
	Capability string
}

func (e *UnsupportedCapabilityError) Error() string {
	return fmt.Sprintf("mcp: unsupported capability %q", e.Capability)
}
