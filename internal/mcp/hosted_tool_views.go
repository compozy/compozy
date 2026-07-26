package mcp

import "github.com/compozy/agh/internal/tools"

func cloneToolViews(src []tools.ToolView) []tools.ToolView {
	if src == nil {
		return nil
	}
	out := make([]tools.ToolView, len(src))
	copy(out, src)
	for i := range out {
		out[i].Descriptor.ToolPresentation = tools.CloneToolPresentation(src[i].Descriptor.ToolPresentation)
		out[i].Descriptor.InputSchema = cloneRaw(out[i].Descriptor.InputSchema)
		out[i].Descriptor.OutputSchema = cloneRaw(out[i].Descriptor.OutputSchema)
		out[i].Descriptor.Toolsets = append([]tools.ToolsetID(nil), out[i].Descriptor.Toolsets...)
		out[i].Descriptor.Tags = append([]string(nil), out[i].Descriptor.Tags...)
		out[i].Descriptor.SearchHints = append([]string(nil), out[i].Descriptor.SearchHints...)
		out[i].Availability.ReasonCodes = append([]tools.ReasonCode(nil), out[i].Availability.ReasonCodes...)
		out[i].Decision.ReasonCodes = append([]tools.ReasonCode(nil), out[i].Decision.ReasonCodes...)
	}
	return out
}
