package windowmanager

import "fmt"

func (r *reducer) navigateWindow(snapshot *Snapshot, command NavigateWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if command.Mode == NavigatePop {
		if command.Route.Pathname != "" || command.Route.Search != nil {
			return false, fmt.Errorf("pop navigation cannot include a route: %w", ErrInvalidCommand)
		}
		if len(window.NavStack) == 0 {
			return false, nil
		}
		window.Route = cloneRouteIntent(window.NavStack[len(window.NavStack)-1])
		window.NavStack = window.NavStack[:len(window.NavStack)-1]
	} else {
		if command.Mode != NavigateReplace && command.Mode != NavigatePush {
			return false, fmt.Errorf("navigation mode %q: %w", command.Mode, ErrInvalidCommand)
		}
		route, err := CanonicalRouteIntent(command.Route)
		if err != nil {
			return false, fmt.Errorf("window %q route: %w: %w", command.WindowID, err, ErrInvalidCommand)
		}
		if routeIntentsEqual(window.Route, route) {
			return false, nil
		}
		if command.Mode == NavigatePush {
			window.NavStack = append(window.NavStack, cloneRouteIntent(window.Route))
			if len(window.NavStack) > r.config.NavStackLimit {
				window.NavStack = append(
					[]RouteIntent(nil),
					window.NavStack[len(window.NavStack)-r.config.NavStackLimit:]...)
			}
		}
		window.Route = route
	}
	snapshot.Windows[command.WindowID] = window
	r.changes.window(command.WindowID)
	return true, nil
}
