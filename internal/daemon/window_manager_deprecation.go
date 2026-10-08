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
