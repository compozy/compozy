package tool

import (
	"path/filepath"
	"strings"
)

// ToolID represents a tool identifier
type ToolID string

// ToolDescription represents a tool description
type ToolDescription string

// ToolExecute represents a tool execution path
type ToolExecute string

// IsTypeScript checks if the tool execution path is a TypeScript file
func (t ToolExecute) IsTypeScript() bool {
	ext := filepath.Ext(string(t))
	return strings.EqualFold(ext, ".ts")
}
