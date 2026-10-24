package windowmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (m *Manager) validateCommandBindings(
	ctx context.Context,
	snapshot Snapshot,
	command Command,
	changes ChangeSet,
) error {
	if snapshot.WorkspaceID != GlobalDesktopWorkspaceID {
		return nil
	}
	switch command.(type) {
	case OpenWindowCommand, NavigateWindowCommand, ReopenCommand, ReplaceLayoutCommand:
	default:
		return nil
	}
	routes := []RouteIntent{}
	switch c := command.(type) {
	case OpenWindowCommand:
		routes = append(routes, c.Window.Route)
	case NavigateWindowCommand:
		routes = append(routes, c.Route)
	case ReplaceLayoutCommand:
		for _, window := range snapshot.Windows {
			routes = append(routes, window.Route)
			routes = append(routes, window.NavStack...)
		}
	}
	for _, id := range changes.WindowIDs {
		if window, ok := snapshot.Windows[id]; ok {
			routes = append(routes, window.Route)
			routes = append(routes, window.NavStack...)
		}
	}
	for _, route := range routes {
		var binding string
		if raw, ok := route.Search["workspace"]; ok {
			if err := json.Unmarshal(raw, &binding); err != nil {
				return fmt.Errorf("%w: workspace must be a string", ErrWindowBindingInvalid)
			}
		}
		binding = strings.TrimSpace(binding)
		if binding == "" || binding == workspaceaccess.UserSelector {
			continue
		}
		if binding == string(GlobalDesktopWorkspaceID) || binding == "user" {
			return fmt.Errorf("%w: %q is not a project binding", ErrWindowBindingInvalid, binding)
		}
		if err := m.workspaces.ResolveWorkspace(ctx, WorkspaceID(binding)); err != nil {
			return fmt.Errorf("%w: workspace %q: %v", ErrWindowBindingInvalid, binding, err)
		}
	}
	return nil
}
