package core

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	"github.com/compozy/compozy/internal/windowmanager"
)

var deprecatedPaletteIDs sync.Map

// Public aliases remain available through their named removal release.
func (h *BaseHandlers) canonicalPaletteCommandID(ctx context.Context, raw string) cmdpalette.CommandID {
	return resolvePaletteCommandID(ctx, h.Logger, &deprecatedPaletteIDs, raw)
}

func resolvePaletteCommandID(
	ctx context.Context,
	logger *slog.Logger,
	warnedIDs *sync.Map,
	raw string,
) cmdpalette.CommandID {
	id := cmdpalette.CommandID(strings.TrimSpace(raw))
	replacement, retired := corecmds.RetiredCommandID(id)
	if retired {
		if _, warned := warnedIDs.LoadOrStore(id, struct{}{}); !warned {
			if logger == nil {
				logger = slog.Default()
			}
			if corecmds.RetiredCommandRemovalRelease(id) == "v0.8.0" {
				windowmanager.WarnSurfaceDeprecation(ctx, logger, windowmanager.SurfaceDeprecation(string(id)), "api")
				return replacement
			}
			logger.WarnContext(ctx, "deprecated command id", "event", "cmdpalette.command_id_deprecated",
				"id", id, "replacement", replacement, "removal", corecmds.RetiredCommandRemovalRelease(id))
		}
	}
	return replacement
}

func (h *BaseHandlers) canonicalPaletteViewID(ctx context.Context, raw string) string {
	id := strings.TrimSpace(raw)
	return strings.TrimPrefix(string(h.canonicalPaletteCommandID(ctx, "palette.view."+id)), "palette.view.")
}
