package windowmanager

import "fmt"

func (r *reducer) navigateWindow(snapshot *Snapshot, command NavigateWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	route, err := CanonicalRouteIntent(command.Route)
	if err != nil {
		return false, fmt.Errorf("window %q route: %w: %w", command.WindowID, err, ErrInvalidCommand)
	}
	if routeIntentsEqual(window.Route, route) {
		return false, nil
	}
	window.Route = route
	snapshot.Windows[command.WindowID] = window
	r.changes.window(command.WindowID)
	return true, nil
}
