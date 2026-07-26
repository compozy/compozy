package tools

import (
	"strings"
	"sync"

	"github.com/compozy/agh/internal/toolmeta"
)

type descriptorMetadataIndex struct {
	mu          sync.RWMutex
	byWorkspace map[string]map[ToolID]toolmeta.DescriptorMetadata
}

var _ toolmeta.DescriptorLookup = (*RuntimeRegistry)(nil)

// LookupToolMetadata returns presentation metadata discovered by the runtime registry.
func (r *RuntimeRegistry) LookupToolMetadata(
	workspaceID string,
	toolID string,
) (toolmeta.DescriptorMetadata, bool) {
	if r == nil {
		return toolmeta.DescriptorMetadata{}, false
	}
	id := ToolID(strings.TrimSpace(toolID))
	if err := id.Validate(); err != nil {
		return toolmeta.DescriptorMetadata{}, false
	}
	return r.descriptorMetadata.lookup(workspaceID, id)
}

func (i *descriptorMetadataIndex) remember(workspaceID string, entries []*registryEntry) {
	if i == nil {
		return
	}
	workspaceID = strings.TrimSpace(workspaceID)
	next := make(map[ToolID]toolmeta.DescriptorMetadata, len(entries))
	for _, entry := range entries {
		if entry == nil || !descriptorMetadataVisibleToWorkspace(entry.descriptor.Source, workspaceID) {
			continue
		}
		descriptor := entry.descriptor
		presentation := descriptor.Presentation()
		next[descriptor.ID] = toolmeta.DescriptorMetadata{
			ID:           descriptor.ID.String(),
			DisplayTitle: descriptor.DisplayTitle,
			FriendlyVerb: presentation.FriendlyVerb,
			Preview:      presentation.Preview,
		}
	}
	i.mu.Lock()
	if i.byWorkspace == nil {
		i.byWorkspace = make(map[string]map[ToolID]toolmeta.DescriptorMetadata)
	}
	i.byWorkspace[workspaceID] = next
	i.mu.Unlock()
}

func (i *descriptorMetadataIndex) lookup(
	workspaceID string,
	id ToolID,
) (toolmeta.DescriptorMetadata, bool) {
	if i == nil {
		return toolmeta.DescriptorMetadata{}, false
	}
	workspaceID = strings.TrimSpace(workspaceID)
	i.mu.RLock()
	defer i.mu.RUnlock()
	metadataByID, indexed := i.byWorkspace[workspaceID]
	if !indexed && workspaceID != "" {
		metadataByID = i.byWorkspace[""]
	}
	metadata, ok := metadataByID[id]
	return metadata, ok
}

func descriptorMetadataVisibleToWorkspace(source SourceRef, workspaceID string) bool {
	sourceWorkspaceID := strings.TrimSpace(source.WorkspaceID)
	workspaceScoped := sourceWorkspaceID != "" || strings.EqualFold(strings.TrimSpace(source.Scope), "workspace")
	if !workspaceScoped {
		return true
	}
	return workspaceID != "" && sourceWorkspaceID == workspaceID
}
