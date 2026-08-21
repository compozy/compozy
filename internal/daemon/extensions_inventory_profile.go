package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) InventoryScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionInventoryPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	if err := ctx.Err(); err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	if err := actor.Validate(); err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionInventoryPayload{}, taskpkg.ErrPermissionDenied
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	ext, err := s.projectExtensionReadProfile(ctx, runtime, extensionpkg.InstanceKey{
		Name:        strings.TrimSpace(name),
		WorkspaceID: workspaceID,
	}, profile)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	desired, err := projectExtensionKitItems(ctx, ext, s.resourceCodecs, s.getenv)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	live, err := s.extensionOwnedResourceRecords(ctx, name)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	live = filterExtensionInventoryRecords(live, workspaceID, profile)
	if !ext.Info.Enabled {
		live = nil
	}
	items := s.mergeExtensionKitInventory(ctx, desired, live)
	projection, err := s.payloadFromExtension(ctx, ext, profile)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	return contract.ExtensionInventoryPayload{
		Extension:   ext.Info.Name,
		Format:      projection.Format,
		Enabled:     ext.Info.Enabled,
		Items:       contractExtensionKitItems(items),
		Diagnostics: append([]contract.DiagnosticItem(nil), projection.Diagnostics...),
	}, nil
}

func filterExtensionInventoryRecords(
	records []resources.RawRecord,
	workspaceID string,
	profile extensionpkg.ProfileLens,
) []resources.RawRecord {
	workspaceID = strings.TrimSpace(workspaceID)
	profileID := strings.TrimSpace(profile.ID)
	workspaceProfileID := workspaceID + "@pf:" + strings.TrimSpace(profile.Name)
	visible := make([]resources.RawRecord, 0, len(records))
	for _, record := range records {
		scope := record.Scope.Normalize()
		isVisible := false
		switch scope.Kind {
		case resources.ResourceScopeKindUser:
			isVisible = true
		case resources.ResourceScopeKindProfile:
			isVisible = scope.ID == profileID
		case resources.ResourceScopeKindWorkspace:
			isVisible = workspaceID != "" && scope.ID == workspaceID
		case resources.ResourceScopeKindWorkspaceProfile:
			isVisible = workspaceID != "" && scope.ID == workspaceProfileID
		}
		if isVisible {
			visible = append(visible, record)
		}
	}
	return visible
}
