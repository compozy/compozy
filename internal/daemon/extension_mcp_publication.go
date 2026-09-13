package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/extensionmcp"
	"github.com/compozy/compozy/internal/resources"
)

func extensionMCPPublicationPreparer(state *bootState) func(context.Context, []toolMCPDesiredResources) error {
	if state == nil || state.extensionMCP == nil {
		return nil
	}
	return func(ctx context.Context, providers []toolMCPDesiredResources) error {
		var manual, extensions []*mcpServerPublicationInput
		for idx := range providers {
			for serverIdx := range providers[idx].mcpServers {
				item := &providers[idx].mcpServers[serverIdx]
				if item.owner != nil && item.owner.Normalize().Kind == extensionResourceOwnerKind {
					extensions = append(extensions, item)
				} else {
					item.spec.Owner, item.spec.RuntimeName = mcpDefinitionOwnerManual, item.spec.Name
					manual = append(manual, item)
				}
			}
		}
		slices.SortFunc(extensions, func(a, b *mcpServerPublicationInput) int {
			return strings.Compare(
				string(a.scope.Kind)+a.scope.ID+a.sourceKey,
				string(b.scope.Kind)+b.scope.ID+b.sourceKey,
			)
		})
		for _, item := range extensions {
			if err := prepareExtensionMCPPublication(ctx, state, item, manual); err != nil {
				return err
			}
		}
		return nil
	}
}

func prepareExtensionMCPPublication(
	ctx context.Context, state *bootState, item *mcpServerPublicationInput, manual []*mcpServerPublicationInput,
) error {
	profileID, workspaceID, err := mcpExtensionBindingOwner(ctx, state, item.scope)
	if err != nil {
		return err
	}
	target := extensionmcp.Target{
		Extension:   item.owner.ID,
		ProfileID:   profileID,
		WorkspaceID: workspaceID,
		ServerName:  item.spec.Name,
	}
	var occupied []string
	for _, entry := range manual {
		if manualMCPVisibleTo(entry.scope, item.scope, workspaceID, profileID) {
			occupied = append(occupied, entry.spec.Name)
		}
	}
	if workspaceID != "" {
		inherited, err := state.extensionMCP.List(ctx, profileID, "")
		if err != nil {
			return err
		}
		for _, entry := range inherited {
			occupied = append(occupied, entry.RuntimeName)
		}
	}
	record, err := state.extensionMCP.Reserve(ctx, target, "", occupied)
	if err != nil {
		return fmt.Errorf("daemon: allocate extension MCP %s/%s: %w", target.Extension, target.ServerName, err)
	}
	item.spec.Owner, item.spec.RuntimeName = "extension:"+target.Extension, record.RuntimeName
	item.spec, err = record.Apply(item.spec)
	return err
}

func manualMCPVisibleTo(manual, target resources.ResourceScope, workspaceID, profileID string) bool {
	manual, target = manual.Normalize(), target.Normalize()
	if manual.Kind == resources.ResourceScopeKindUser || manual == target {
		return true
	}
	if manual.Kind == resources.ResourceScopeKindWorkspace {
		return workspaceID != "" && manual.ID == workspaceID
	}
	return manual.Kind == resources.ResourceScopeKindProfile && manual.ID == profileID
}
