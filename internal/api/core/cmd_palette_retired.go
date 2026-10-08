package core

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
)

var deprecatedPaletteIDs sync.Map

// Public aliases are removed in v0.5.0.
func (h *BaseHandlers) canonicalPaletteCommandID(raw string) cmdpalette.CommandID {
	return resolvePaletteCommandID(h.Logger, &deprecatedPaletteIDs, raw)
}

func resolvePaletteCommandID(logger *slog.Logger, warnedIDs *sync.Map, raw string) cmdpalette.CommandID {
	id := cmdpalette.CommandID(strings.TrimSpace(raw))
	replacement, retired := corecmds.RetiredCommandID(id)
	if retired {
		if _, warned := warnedIDs.LoadOrStore(id, struct{}{}); !warned {
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("deprecated command id", "event", "cmdpalette.command_id_deprecated",
				"id", id, "replacement", replacement, "removal", "v0.5.0")
		}
	}
	return replacement
}

func (h *BaseHandlers) canonicalPaletteViewID(raw string) string {
	id := strings.TrimSpace(raw)
	return strings.TrimPrefix(string(h.canonicalPaletteCommandID("palette.view."+id)), "palette.view.")
}
