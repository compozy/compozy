package tool

import (
	"path/filepath"
	"strings"
)

type ToolID string
type ToolDescription string
type ToolExecute string

func (t ToolExecute) IsTypeScript() bool {
	ext := filepath.Ext(string(t))
	return strings.EqualFold(ext, ".ts")
}
