package daemon

import (
	"slices"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func cloneToolSpec(src toolspkg.Tool) toolspkg.Tool {
	cloned := src
	cloned.ToolPresentation = toolspkg.CloneToolPresentation(src.ToolPresentation)
	if len(src.InputSchema) > 0 {
		cloned.InputSchema = append([]byte(nil), src.InputSchema...)
	}
	if len(src.OutputSchema) > 0 {
		cloned.OutputSchema = append([]byte(nil), src.OutputSchema...)
	}
	cloned.Backend.RequiresCapabilities = slices.Clone(src.Backend.RequiresCapabilities)
	cloned.Toolsets = slices.Clone(src.Toolsets)
	cloned.Tags = slices.Clone(src.Tags)
	cloned.SearchHints = slices.Clone(src.SearchHints)
	return cloned
}
