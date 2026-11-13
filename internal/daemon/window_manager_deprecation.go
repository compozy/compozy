package daemon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/compozy/compozy/internal/windowmanager"
)

var deprecatedWindowApps sync.Map

// Public app-id warnings are removed with the aliases in v0.5.0.
func windowManagerAppDeprecationLogger(
	logger *slog.Logger, warnedApps *sync.Map,
) windowmanager.AppDeprecationObserver {
	return func(ctx context.Context, app, replacement, source string) {
		if _, warned := warnedApps.LoadOrStore(app, struct{}{}); warned {
			return
		}
		logger.WarnContext(ctx, "deprecated app id", "event", "windowmanager.app_id_deprecated",
			"app", app, "replacement", replacement, "removal", "v0.5.0", "source", source)
	}
}

var deprecatedWindowSurfaces sync.Map

// Remove surface warning deduplication with the shims in v0.8.0.
func windowManagerSurfaceDeprecationLogger(logger *slog.Logger, seen *sync.Map) windowmanager.DeprecationObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, request windowmanager.CommandRequest, entries []windowmanager.Deprecation) {
		for _, entry := range entries {
			if _, warned := seen.LoadOrStore(entry.Surface, struct{}{}); warned {
				continue
			}
			logger.WarnContext(
				ctx,
				"window_manager.deprecated_surface_used",
				"surface",
				entry.Surface,
				"replacement",
				entry.Replacement,
				"actor_kind",
				request.Actor.Kind,
				"removal_release",
				entry.RemovalRelease,
			)
		}
	}
}
