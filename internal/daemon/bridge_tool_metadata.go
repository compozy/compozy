package daemon

import "github.com/compozy/agh/internal/toolmeta"

type bridgeToolMetadataLookup struct {
	state *bootState
}

var _ toolmeta.DescriptorLookup = bridgeToolMetadataLookup{}

func (l bridgeToolMetadataLookup) LookupToolMetadata(
	workspaceID string,
	toolID string,
) (toolmeta.DescriptorMetadata, bool) {
	if l.state == nil || l.state.toolRegistry == nil {
		return toolmeta.DescriptorMetadata{}, false
	}
	lookup, ok := l.state.toolRegistry.(toolmeta.DescriptorLookup)
	if !ok {
		return toolmeta.DescriptorMetadata{}, false
	}
	return lookup.LookupToolMetadata(workspaceID, toolID)
}
