package settings

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

type cmdPaletteSettingsEventNotifier interface {
	NotifyBindingChanged(context.Context, cmdpalette.ProfileLens, cmdpalette.WorkspaceID, cmdpalette.CommandID)
	NotifyAliasChanged(context.Context, cmdpalette.ProfileLens, cmdpalette.WorkspaceID, cmdpalette.CommandID)
}

func changedBindingCommandIDs(
	current compozyconfig.WindowManagerConfig,
	desired compozyconfig.WindowManagerConfig,
) []cmdpalette.CommandID {
	ids := make(map[string]struct{}, len(current.Shortcuts)+len(desired.Shortcuts)+
		len(current.GlobalShortcuts)+len(desired.GlobalShortcuts))
	collectMapKeys(ids, current.Shortcuts)
	collectMapKeys(ids, desired.Shortcuts)
	collectMapKeys(ids, current.GlobalShortcuts)
	collectMapKeys(ids, desired.GlobalShortcuts)
	changed := make([]cmdpalette.CommandID, 0, len(ids))
	for id := range ids {
		if !reflect.DeepEqual(current.Shortcuts[id], desired.Shortcuts[id]) ||
			current.GlobalShortcuts[id] != desired.GlobalShortcuts[id] {
			changed = append(changed, cmdpalette.CommandID(id))
		}
	}
	slices.Sort(changed)
	return changed
}

func changedAliasCommandIDs(current, desired map[string]string) []cmdpalette.CommandID {
	ids := make(map[string]struct{}, len(current)+len(desired))
	collectMapKeys(ids, current)
	collectMapKeys(ids, desired)
	changed := make([]cmdpalette.CommandID, 0, len(ids))
	for id := range ids {
		if current[id] != desired[id] {
			changed = append(changed, cmdpalette.CommandID(id))
		}
	}
	slices.Sort(changed)
	return changed
}

func collectMapKeys[V any](target map[string]struct{}, source map[string]V) {
	for key := range source {
		target[key] = struct{}{}
	}
}

func (s *service) cmdPaletteEventWorkspaces(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	hasChanges bool,
) ([]cmdpalette.WorkspaceID, error) {
	if !hasChanges {
		return nil, nil
	}
	if _, ok := s.cmdPaletteNotifier(); !ok {
		return nil, nil
	}
	if scope == ScopeWorkspace {
		return []cmdpalette.WorkspaceID{cmdpalette.WorkspaceID(workspaceID)}, nil
	}
	if s.workspaceResolver == nil {
		return nil, nil
	}
	workspaces, err := s.workspaceResolver.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: list workspaces for command palette events: %w", err)
	}
	ids := make([]cmdpalette.WorkspaceID, 0, len(workspaces))
	seen := make(map[string]struct{}, len(workspaces))
	for _, workspace := range workspaces {
		id := strings.TrimSpace(workspace.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, cmdpalette.WorkspaceID(id))
	}
	slices.Sort(ids)
	return ids, nil
}

func (s *service) cmdPaletteNotifier() (cmdPaletteSettingsEventNotifier, bool) {
	notifier, ok := s.cmdPalette.(cmdPaletteSettingsEventNotifier)
	return notifier, ok
}

func (s *service) emitCmdPaletteSettingsEvents(
	ctx context.Context,
	workspaces []cmdpalette.WorkspaceID,
	bindingChanges []cmdpalette.CommandID,
	aliasChanges []cmdpalette.CommandID,
) {
	notifier, ok := s.cmdPaletteNotifier()
	if !ok {
		return
	}
	for _, workspaceID := range workspaces {
		// Base settings changes affect every profile. An aggregate lens lets
		// profile-scoped subscribers invalidate their effective catalog too.
		profileLens := cmdpalette.AggregateProfileLens()
		for _, commandID := range bindingChanges {
			notifier.NotifyBindingChanged(ctx, profileLens, workspaceID, commandID)
		}
		for _, commandID := range aliasChanges {
			notifier.NotifyAliasChanged(ctx, profileLens, workspaceID, commandID)
		}
	}
}
