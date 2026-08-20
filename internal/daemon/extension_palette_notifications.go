package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type cmdPaletteCatalogNotifier interface {
	NotifyCatalogChanged(context.Context, cmdpalette.WorkspaceID) error
}

type extensionPaletteNotifier struct {
	catalog    func() cmdpalette.Registry
	views      func() cmdpalette.ViewSessionService
	workspaces func(context.Context) ([]workspacepkg.Workspace, error)
}

func newExtensionPaletteNotifier(state *bootState) *extensionPaletteNotifier {
	if state == nil {
		return nil
	}
	return &extensionPaletteNotifier{
		catalog: func() cmdpalette.Registry { return state.cmdPalette },
		views: func() cmdpalette.ViewSessionService {
			service, ok := state.cmdPalette.(cmdpalette.ViewSessionService)
			if !ok {
				return nil
			}
			return service
		},
		workspaces: func(ctx context.Context) ([]workspacepkg.Workspace, error) {
			if state.workspaceResolver == nil {
				return nil, nil
			}
			return state.workspaceResolver.List(ctx)
		},
	}
}

func (n *extensionPaletteNotifier) Notify(ctx context.Context, workspaceID string) error {
	if n == nil {
		return nil
	}
	workspaces, err := n.targetWorkspaces(ctx, workspaceID)
	if err != nil {
		return err
	}
	return n.notifyCatalogs(ctx, workspaces)
}

func (n *extensionPaletteNotifier) NotifyExtensionChanged(
	ctx context.Context,
	workspaceID string,
	extension string,
) error {
	if n == nil {
		return nil
	}
	workspaces, err := n.targetWorkspaces(ctx, workspaceID)
	if err != nil {
		return err
	}
	return errors.Join(
		n.notifyCatalogs(ctx, workspaces),
		n.invalidateViewSessions(ctx, workspaces, extension),
	)
}

func (n *extensionPaletteNotifier) targetWorkspaces(
	ctx context.Context,
	workspaceID string,
) ([]cmdpalette.WorkspaceID, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID != "" {
		return []cmdpalette.WorkspaceID{cmdpalette.WorkspaceID(workspaceID)}, nil
	}
	if n.workspaces == nil {
		return nil, nil
	}
	workspaces, err := n.workspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list workspaces for extension palette notification: %w", err)
	}
	ids := make([]cmdpalette.WorkspaceID, 0, len(workspaces))
	for _, workspace := range workspaces {
		ids = append(ids, cmdpalette.WorkspaceID(workspace.ID))
	}
	return ids, nil
}

func (n *extensionPaletteNotifier) notifyCatalogs(
	ctx context.Context,
	workspaces []cmdpalette.WorkspaceID,
) error {
	if n.catalog == nil {
		return nil
	}
	notifier, ok := n.catalog().(cmdPaletteCatalogNotifier)
	if !ok || notifier == nil {
		return nil
	}
	var notifyErr error
	for _, workspaceID := range workspaces {
		if err := notifier.NotifyCatalogChanged(ctx, workspaceID); err != nil {
			notifyErr = errors.Join(notifyErr, fmt.Errorf(
				"notify workspace %q: %w",
				workspaceID,
				err,
			))
		}
	}
	if notifyErr != nil {
		return fmt.Errorf("daemon: notify extension palette catalogs: %w", notifyErr)
	}
	return nil
}

func (n *extensionPaletteNotifier) invalidateViewSessions(
	ctx context.Context,
	workspaces []cmdpalette.WorkspaceID,
	extension string,
) error {
	if n.views == nil || strings.TrimSpace(extension) == "" {
		return nil
	}
	views := n.views()
	if views == nil {
		return nil
	}
	var invalidateErr error
	for _, workspaceID := range workspaces {
		if err := views.InvalidateInstance(ctx, workspaceID, extension, 0); err != nil {
			invalidateErr = errors.Join(invalidateErr, fmt.Errorf(
				"invalidate %q in workspace %q: %w",
				extension,
				workspaceID,
				err,
			))
		}
	}
	if invalidateErr != nil {
		return fmt.Errorf("daemon: invalidate extension view sessions: %w", invalidateErr)
	}
	return nil
}

type extensionPaletteLifecycleEventSink struct {
	primary  extensionpkg.LifecycleEventSink
	notifier *extensionPaletteNotifier
}

func (s extensionPaletteLifecycleEventSink) RecordExtensionLifecycleEvent(
	ctx context.Context,
	event extensionpkg.LifecycleEvent,
) error {
	if s.primary != nil {
		if err := s.primary.RecordExtensionLifecycleEvent(ctx, event); err != nil {
			return err
		}
	}
	if event.Type != eventspkg.ExtensionCrashLoopBackoff {
		return nil
	}
	return s.notifier.NotifyExtensionChanged(ctx, event.WorkspaceID, event.ExtensionName)
}

func extensionLifecycleChangesPalette(eventType string) bool {
	switch eventType {
	case eventspkg.ExtensionEnabled,
		eventspkg.ExtensionDisabled,
		eventspkg.ExtensionRemoved,
		eventspkg.ExtensionDevLinked,
		eventspkg.ExtensionDevUnlinked,
		eventspkg.ExtensionReloadCompleted:
		return true
	default:
		return false
	}
}
