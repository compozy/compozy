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
	workspaces func(context.Context) ([]workspacepkg.Workspace, error)
}

func newExtensionPaletteNotifier(state *bootState) *extensionPaletteNotifier {
	if state == nil {
		return nil
	}
	return &extensionPaletteNotifier{
		catalog: func() cmdpalette.Registry { return state.cmdPalette },
		workspaces: func(ctx context.Context) ([]workspacepkg.Workspace, error) {
			if state.workspaceResolver == nil {
				return nil, nil
			}
			return state.workspaceResolver.List(ctx)
		},
	}
}

func (n *extensionPaletteNotifier) Notify(ctx context.Context, workspaceID string) error {
	if n == nil || n.catalog == nil {
		return nil
	}
	notifier, ok := n.catalog().(cmdPaletteCatalogNotifier)
	if !ok || notifier == nil {
		return nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID != "" {
		if err := notifier.NotifyCatalogChanged(ctx, cmdpalette.WorkspaceID(workspaceID)); err != nil {
			return fmt.Errorf("daemon: notify extension palette catalog for workspace %q: %w", workspaceID, err)
		}
		return nil
	}
	if n.workspaces == nil {
		return nil
	}
	workspaces, err := n.workspaces(ctx)
	if err != nil {
		return fmt.Errorf("daemon: list workspaces for extension palette notification: %w", err)
	}
	var notifyErr error
	for _, workspace := range workspaces {
		if err := notifier.NotifyCatalogChanged(ctx, cmdpalette.WorkspaceID(workspace.ID)); err != nil {
			notifyErr = errors.Join(notifyErr, err)
		}
	}
	if notifyErr != nil {
		return fmt.Errorf("daemon: notify extension palette catalogs: %w", notifyErr)
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
	return s.notifier.Notify(ctx, event.WorkspaceID)
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
