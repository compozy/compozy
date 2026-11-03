package config

import (
	"log/slog"
	"sync"
)

// deprecationWarnings routes the v0.5.1 input-alias warnings (agent kind, config table, config
// paths, request kind) to the daemon logger. Removal with the aliases: v0.5.2.
var deprecationWarnings struct {
	mu     sync.Mutex
	logger *slog.Logger
}

// RouteDeprecationWarnings sends later alias deprecation warnings to logger and re-arms their
// once-per-process deduplication. A nil logger restores slog.Default.
func RouteDeprecationWarnings(logger *slog.Logger) {
	deprecationWarnings.mu.Lock()
	deprecationWarnings.logger = logger
	deprecationWarnings.mu.Unlock()
	agentKindAliasWarnings.Clear()
	configTableAliasWarnings.Clear()
}

// DeprecationLogger returns the logger that receives alias deprecation warnings.
func DeprecationLogger() *slog.Logger {
	deprecationWarnings.mu.Lock()
	defer deprecationWarnings.mu.Unlock()
	if deprecationWarnings.logger == nil {
		return slog.Default()
	}
	return deprecationWarnings.logger
}
